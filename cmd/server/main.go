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

    "github.com/blagoweb/bbtg/internal/config"
    "github.com/blagoweb/bbtg/internal/db"
    "github.com/blagoweb/bbtg/internal/handler"
    r2storage "github.com/blagoweb/bbtg/internal/storage/r2"
    "github.com/blagoweb/bbtg/internal/telegram"
)

// CORS middleware для обработки cross-origin запросов
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
    fmt.Println(allowedOrigins)
    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")
        allowed := false
        
        for _, allowedOrigin := range allowedOrigins {
            if origin == allowedOrigin {
                allowed = true
                break
            }
        }
        
        if allowed {
            c.Header("Access-Control-Allow-Origin", origin)
        }
        
        c.Header("Access-Control-Allow-Credentials", "true")
        c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
        c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    }
}

// HandleLogin обрабатывает авторизацию через Telegram WebApp
func HandleLogin(telegramToken, jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            InitData string `json:"initData" binding:"required"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        // Проверяем подпись данных от Telegram
        data, err := telegram.CheckAuthData(req.InitData, telegramToken)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram data"})
            return
        }

        // Генерируем JWT токен
        token, err := telegram.GenerateJWT(data, jwtSecret)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
            return
        }

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

// WebhookHandler обрабатывает webhook от YooKassa
func WebhookHandler(db interface{}, yookassaSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // TODO: Implement YooKassa webhook handling
        c.JSON(http.StatusOK, gin.H{"status": "ok"})
    }
}

func main() {
    // 1. Загрузить конфиг
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
    }

    // 2. Соединиться с БД
    database, err := db.Connect(cfg.DB_DSN)
    if err != nil {
        log.Fatalf("db connect error: %v", err)
    }

    // 3. Запустить миграции (опционально)
    if err := runMigrations(cfg.DB_DSN); err != nil {
        log.Printf("migrations warning: %v", err)
        // Не прерываем выполнение, если миграции уже применены
    }

    // 4. Инициализировать R2-клиент
    r2client, err := r2storage.NewClient(cfg.R2Endpoint, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2Bucket)
    if err != nil {
        log.Fatalf("R2 init error: %v", err)
    }

    // 5. Инициализировать Telegram-бота
    tbot, err := telegram.NewBot(cfg.TelegramToken)
    if err != nil {
        log.Fatalf("telegram bot init error: %v", err)
    }

    // 6. Настроить Gin
    router := gin.Default()

    // --- автоматически обрабатывает OPTIONS и проставляет все CORS-заголовки ---
    router.Use(cors.New(cors.Config{
        AllowOrigins:     cfg.CORSOrigins,             // из вашего config.Load()
        AllowMethods:     []string{"GET","POST","PUT","DELETE","OPTIONS"},
        AllowHeaders:     []string{"Origin","Content-Type","Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }))

    // Health check endpoint для Railway
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "ok"})
    })

    // Авторизация через Telegram WebApp
    router.POST("/api/auth/login", HandleLogin(cfg.TelegramToken, cfg.JWTSecret))

    // Вебхук оплаты YooKassa
    router.POST("/api/payment/webhook", WebhookHandler(database, cfg.YookassaSecret))

    // Группа защищённых API-маршрутов
    api := router.Group("/api")
    api.Use(AuthMiddleware(cfg.JWTSecret))
    {
        // лендинги (CRUD + загрузка аватарки)
        handler.RegisterLandingRoutes       (api, database, r2client)
        // ссылки/кнопки
        handler.RegisterLinkRoutes          (api, database)
        // заявки (лиды)
        handler.RegisterLeadRoutes          (api, database, tbot)
        // аналитика (просмотры, клики)
        handler.RegisterAnalyticsRoutes     (api, database)
        // списки платежей
        handler.RegisterPaymentRoutes       (api, database)
        // подписки (рекуррентные платежи)
        handler.RegisterSubscriptionRoutes  (api, database, cfg)
    }

    // 7. Запустить HTTP-сервер
    addr := fmt.Sprintf(":%s", cfg.AppPort)
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