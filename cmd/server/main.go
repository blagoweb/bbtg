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
            log.Printf("Login request binding error: %v", err)
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        // Проверяем подпись данных от Telegram
        data, err := telegram.CheckAuthData(req.InitData, telegramToken)
        if err != nil {
            log.Printf("Telegram auth data validation failed: %v", err)
            log.Printf("InitData received: %s", req.InitData)
            log.Printf("Telegram token configured: %s", telegramToken)
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram data"})
            return
        }

        // Генерируем JWT токен
        token, err := telegram.GenerateJWT(data, jwtSecret)
        if err != nil {
            log.Printf("JWT generation failed: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
            return
        }

        log.Printf("Login successful for user: %s", data["user_id"])
        c.JSON(http.StatusOK, gin.H{"token": token})
    }
}

// AuthMiddleware проверяет JWT токен и добавляет user_id в контекст
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "no authorization header"})
            c.Abort()
            return
        }

        // Убираем "Bearer " префикс
        tokenString := authHeader
        if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
            tokenString = authHeader[7:]
        }

        // Парсим JWT токен
        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return []byte(jwtSecret), nil
        })

        if err != nil || !token.Valid {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }

        // Извлекаем user_id из токена
        if claims, ok := token.Claims.(jwt.MapClaims); ok {
            if userID, exists := claims["user_id"]; exists {
                // Преобразуем user_id в строку для консистентности
                c.Set("user_id", fmt.Sprint(userID))
                c.Next()
                return
            }
        }

        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
        c.Abort()
    }
}

func main() {
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
    corsOrigins := "*"

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
    }

    // 3. Миграции (опционально)
    if dbDSN != "" {
        if err := runMigrations(dbDSN); err != nil {
            log.Printf("migrations warning: %v", err)
        }
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
    
    // Расширяем CORS origins для Railway
    corsOriginsList := []string{corsOrigins}
    if corsOrigins != "*" {
        // Добавляем дополнительные origins для отладки
        corsOriginsList = append(corsOriginsList, "*")
    }
    
    router.Use(cors.New(cors.Config{
        AllowOrigins:     corsOriginsList,
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
        ExposeHeaders:    []string{"Content-Length", "Content-Type"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }))
    
    // Логирование всех запросов для отладки
    router.Use(func(c *gin.Context) {
        log.Printf("Request: %s %s from %s", c.Request.Method, c.Request.URL.Path, c.Request.Header.Get("Origin"))
        c.Next()
    })

    // Глобальный обработчик для OPTIONS запросов
    router.OPTIONS("/*path", func(c *gin.Context) {
        log.Printf("Global OPTIONS handler for: %s", c.Request.URL.Path)
        c.Status(http.StatusOK)
    })

    // 7. Routes
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now().Unix()})
    })
    
    // Эндпоинты авторизации (БЕЗ AuthMiddleware)
    router.POST("/api/auth/login", HandleLogin(telegramToken, jwtSecret))

    // API эндпоинты с авторизацией
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