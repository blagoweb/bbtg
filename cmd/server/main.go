package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/gin-contrib/cors"
    "github.com/gin-gonic/gin"
    "github.com/dgrijalva/jwt-go"
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    _ "github.com/lib/pq"
    "github.com/joho/godotenv"

    "github.com/blagoweb/bbtg/internal/db"
    "github.com/blagoweb/bbtg/internal/handler"
    r2storage "github.com/blagoweb/bbtg/internal/storage/r2"
    "github.com/blagoweb/bbtg/internal/telegram"
)

// HandleLogin обрабатывает авторизацию через Telegram WebApp
func HandleLogin(telegramToken, jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            InitData string `json:"initData" binding:"required"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            log.Printf("HandleLogin: JSON bind error: %v", err)
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        log.Printf("HandleLogin: Received initData: %s", req.InitData)

        // Проверяем подпись данных от Telegram
        data, err := telegram.CheckAuthData(req.InitData, telegramToken)
        if err != nil {
            log.Printf("HandleLogin: Telegram auth check error: %v", err)
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram data"})
            return
        }

        log.Printf("HandleLogin: Telegram auth successful, user data: %+v", data)

        // Генерируем JWT токен
        token, err := telegram.GenerateJWT(data, jwtSecret)
        if err != nil {
            log.Printf("HandleLogin: JWT generation error: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
            return
        }

        log.Printf("HandleLogin: JWT token generated successfully")
        c.JSON(http.StatusOK, gin.H{"token": token})
    }
}

// AuthMiddleware проверяет JWT токен и добавляет user_id в контекст
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        log.Printf("AuthMiddleware: Authorization header: %s", authHeader)
        
        if authHeader == "" {
            log.Printf("AuthMiddleware: No authorization header")
            c.JSON(http.StatusUnauthorized, gin.H{"error": "no authorization header"})
            c.Abort()
            return
        }

        // Убираем "Bearer " префикс
        tokenString := authHeader
        if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
            tokenString = authHeader[7:]
        }
        
        log.Printf("AuthMiddleware: Token string: %s", tokenString)

        // Парсим JWT токен
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return []byte(jwtSecret), nil
        })

        if err != nil {
            log.Printf("AuthMiddleware: JWT parse error: %v", err)
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }
        
        if !token.Valid {
            log.Printf("AuthMiddleware: Token is not valid")
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }

        // Извлекаем telegram_id из токена
        if claims, ok := token.Claims.(jwt.MapClaims); ok {
            if telegramID, exists := claims["telegram_id"]; exists {
                // Преобразуем telegram_id в строку для консистентности
                telegramIDStr := fmt.Sprint(telegramID)
                log.Printf("AuthMiddleware: Telegram ID extracted: %s", telegramIDStr)
                c.Set("telegram_id", telegramIDStr)
                c.Next()
                return
            }
        }

        log.Printf("AuthMiddleware: Invalid token claims")
        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
        c.Abort()
    }
}

func main() {
    // Загружаем .env файл
    if err := godotenv.Load(); err != nil {
        log.Printf("Warning: .env file not found: %v", err)
    }

    // 1. Environment variables
    dbDSN := os.Getenv("DB_DSN")
    telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    jwtSecret := os.Getenv("JWT_SECRET")
    if jwtSecret == "" {
        jwtSecret = "default-secret"
    }
    appPort := os.Getenv("APP_PORT")
    if appPort == "" {
        appPort = "8080"
    }
    corsOrigins := os.Getenv("CORS_ORIGINS")
    if corsOrigins == "" {
        corsOrigins = "*"
    }

    // Логируем переменные окружения для отладки
    log.Printf("Environment variables:")
    log.Printf("  DB_DSN: %s", dbDSN)
    log.Printf("  TELEGRAM_BOT_TOKEN: %s", telegramToken)
    log.Printf("  JWT_SECRET: %s", jwtSecret)
    log.Printf("  APP_PORT: %s", appPort)
    log.Printf("  CORS_ORIGINS: %s", corsOrigins)

    // 2. База
    database, err := db.Connect(dbDSN)
    if err != nil {
        log.Printf("db connect error: %v", err)
        // Не прерываем выполнение, если БД недоступна
        database = nil
    } else {
        log.Printf("Database connected successfully")
    }

    // 3. Миграции (опционально)
    if dbDSN != "" {
        if err := runMigrations(dbDSN); err != nil {
            log.Printf("migrations warning: %v", err)
        } else {
            log.Printf("Migrations completed successfully")
        }
    } else {
        log.Printf("DB_DSN not provided, skipping migrations")
    }

    // 4. R2
    var r2client *r2storage.Client
    r2Endpoint := os.Getenv("R2_ENDPOINT")
    r2AccessKey := os.Getenv("R2_ACCESS_KEY")
    r2SecretKey := os.Getenv("R2_SECRET_KEY")
    r2Bucket := os.Getenv("R2_BUCKET")
    if r2Endpoint != "" && r2AccessKey != "" && r2SecretKey != "" && r2Bucket != "" {
        r2client, err = r2storage.NewClient(r2Endpoint, r2AccessKey, r2SecretKey, r2Bucket)
        if err != nil {
            log.Printf("r2 init error: %v", err)
            r2client = nil
        }
    } else {
        log.Printf("R2 credentials not provided, skipping R2 initialization")
        r2client = nil
    }

    // 5. Telegram Bot
    var tbot *telegram.Bot
    if telegramToken != "" {
        tbot, err = telegram.NewBot(telegramToken)
        if err != nil {
            log.Printf("telegram bot init error: %v", err)
            tbot = nil
        }
    } else {
        log.Printf("Telegram token not provided, skipping Telegram bot initialization")
        tbot = nil
    }

    // 6. Gin + CORS
    router := gin.Default()
    router.Use(cors.New(cors.Config{
        AllowOrigins:     []string{corsOrigins},
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }))

    // 7. Routes
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().Unix()})
    })
    router.POST("/api/auth/login",      HandleLogin(telegramToken, jwtSecret))
    
    // Простой тестовый эндпоинт без базы данных
    router.GET("/api/test", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"message": "API работает без авторизации"})
    })

    // Тестовый эндпоинт для отладки (только для разработки)
    router.POST("/api/auth/test-login", func(c *gin.Context) {
        // Создаем тестового пользователя в базе данных
        var userID int
        err := database.Get(&userID, "SELECT id FROM users WHERE telegram_id=$1", 12345)
        if err != nil {
            // Пользователь не существует, создаем его
            err = database.Get(&userID, "INSERT INTO users(telegram_id, username, created_at) VALUES($1, $2, NOW()) RETURNING id", 12345, "test_user")
            if err != nil {
                log.Printf("Test login: Failed to create user: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
                return
            }
            log.Printf("Test login: Created user with ID: %d", userID)
        } else {
            log.Printf("Test login: Found existing user with ID: %d", userID)
        }

        // Генерируем тестовый JWT токен
        claims := jwt.MapClaims{
            "telegram_id": "12345", // Используем telegram_id вместо user_id
            "username": "test_user",
            "exp":      time.Now().Add(24 * time.Hour).Unix(),
        }
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        tokenString, err := token.SignedString([]byte(jwtSecret))
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
            return
        }
        log.Printf("Test login: Generated token for telegram_id: 12345")
        c.JSON(http.StatusOK, gin.H{"token": tokenString})
    })

    api := router.Group("/api")
    api.Use(AuthMiddleware(jwtSecret))
    {
        handler.RegisterLandingRoutes      (api, database, r2client)
        handler.RegisterLinkRoutes         (api, database)
        handler.RegisterLeadRoutes         (api, database, tbot)
        handler.RegisterAnalyticsRoutes    (api, database)
        handler.RegisterPaymentRoutes      (api, database)
        handler.RegisterSubscriptionRoutes (api, database, nil)
    }

    // 8. Запуск на порту из окружения (Railway) или из конфигурации
    port := os.Getenv("PORT")
    if port == "" {
        port = appPort
    }
    addr := fmt.Sprintf(":%s", port)
    log.Printf("Server running on %s", addr)
    if err := router.Run(addr); err != nil {
        log.Fatalf("server error: %v", err)
    }
}

// Пример функции запуска миграций через golang-migrate
func runMigrations(dsn string) error {
    // Получаем текущую рабочую директорию
    workDir, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("failed to get working directory: %v", err)
    }
    
    m, err := migrate.New(
        fmt.Sprintf("file://%s/migrations", workDir),
        dsn,
    )
    if err != nil {
        return err
    }
    defer m.Close()
    return m.Up()
}