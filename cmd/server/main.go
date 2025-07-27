package main

import (
    "fmt"
    "log"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    "github.com/spf13/viper"

    "your_project/internal/config"
    "your_project/internal/db"
    "your_project/internal/payment"
    r2storage "your_project/internal/storage/r2"
    "your_project/internal/telegram"
    "your_project/internal/handler"
)

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
        log.Fatalf("migrations failed: %v", err)
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
    router.POST("/api/payment/webhook", payment.WebhookHandler(database, cfg.YookassaSecret))
    router.POST("/api/auth/login", handler.HandleLogin(cfg.TelegramToken, cfg.JWTSecret))

    api := router.Group("/api")
    api.Use(handler.AuthMiddleware(cfg.JWTSecret))
    {
        handler.RegisterLandingRoutes(api, database, r2client)
        handler.RegisterLinkRoutes   (api, database)
        handler.RegisterLeadRoutes   (api, database, tbot)
        handler.RegisterAnalyticsRoutes(api, database)
        handler.RegisterPaymentRoutes(api, database)
        handler.RegisterSubscriptionRoutes(api, database, cfg)
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
    m, err := migrate.New(
        "file://migrations",
        dsn,
    )
    if err != nil {
        return err
    }
    defer m.Close()
    return m.Up()
}