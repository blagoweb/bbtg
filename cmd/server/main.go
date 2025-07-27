package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/gin-contrib/cors"
    "github.com/gin-gonic/gin"
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"

    "github.com/blagoweb/bbtg/internal/config"
    "github.com/blagoweb/bbtg/internal/db"
    "github.com/blagoweb/bbtg/internal/handler"
    r2storage "github.com/blagoweb/bbtg/internal/storage/r2"
    "github.com/blagoweb/bbtg/internal/payment"
    "github.com/blagoweb/bbtg/internal/telegram"
)

func main() {
    // 1. Конфиг
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("config load error: %v", err)
    }

    // 2. База
    database, err := db.Connect(cfg.DB_DSN)
    if err != nil {
        log.Fatalf("db connect error: %v", err)
    }

    // 3. Миграции (опционально)
    if err := runMigrations(cfg.DB_DSN); err != nil {
        log.Printf("migrations warning: %v", err)
    }

    // 4. R2
    r2client, err := r2storage.NewClient(cfg.R2Endpoint, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2Bucket)
    if err != nil {
        log.Fatalf("r2 init error: %v", err)
    }

    // 5. Telegram Bot
    tbot, err := telegram.NewBot(cfg.TelegramToken)
    if err != nil {
        log.Fatalf("telegram bot init error: %v", err)
    }

    // 6. Gin + CORS
    router := gin.Default()
    router.Use(cors.New(cors.Config{
        AllowOrigins:     cfg.CORSOrigins,
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }))

    // 7. Routes
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "ok"})
    })
    router.POST("/api/auth/login",      handler.HandleLogin(cfg.TelegramToken, cfg.JWTSecret))
    router.POST("/api/payment/webhook", payment.WebhookHandler(database, cfg.YookassaSecret))

    api := router.Group("/api")
    api.Use(handler.AuthMiddleware(cfg.JWTSecret))
    {
        handler.RegisterLandingRoutes      (api, database, r2client)
        handler.RegisterLinkRoutes         (api, database)
        handler.RegisterLeadRoutes         (api, database, tbot)
        handler.RegisterAnalyticsRoutes    (api, database)
        handler.RegisterPaymentRoutes      (api, database)
        handler.RegisterSubscriptionRoutes (api, database, cfg)
    }

    // 8. Запуск на порту из окружения (Railway) или из конфигурации
    port := os.Getenv("PORT")
    if port == "" {
        port = cfg.AppPort
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