// internal/config/config.go
package config

import (
    "fmt"
    "log"
    "strings"
    "github.com/spf13/viper"
)

// Config хранит настройки приложения
type Config struct {
    AppPort        string   // порт, на котором запускается сервер
    DB_DSN         string   // строка подключения к PostgreSQL
    TelegramToken  string   // токен Telegram-бота
    YookassaSecret string   // секрет для верификации вебхуков YooKassa
    R2Endpoint     string   // endpoint Cloudflare R2
    R2AccessKey    string   // ключ доступа R2
    R2SecretKey    string   // секретный ключ R2
    R2Bucket       string   // имя бакета R2
    JWTSecret      string   // секрет для подписи JWT
    CORSOrigins    []string // разрешённые CORS домены
}

// Load загружает конфигурацию из переменных окружения и (опционально) файла config.yaml
func Load() (*Config, error) {
    viper.SetEnvPrefix("TMA")
    viper.AutomaticEnv()

    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    _ = viper.ReadInConfig() // игнорируем ошибку, если файла нет

    // Парсим CORS origins из переменной окружения (разделённые запятыми)
    corsOriginsStr := viper.GetString("CORS_ORIGINS")
    var corsOrigins []string
    if corsOriginsStr != "" {
        corsOrigins = strings.Split(corsOriginsStr, ",")
        // Убираем пробелы
        for i, origin := range corsOrigins {
            corsOrigins[i] = strings.TrimSpace(origin)
        }
    } else {
        corsOrigins = []string{
            "http://localhost:5173",
            "https://tma-alpha.vercel.app",
        }
    }

    cfg := &Config{
        AppPort:        viper.GetString("APP_PORT"),
        DB_DSN:         viper.GetString("DB_DSN"),
        TelegramToken:  viper.GetString("TELEGRAM_BOT_TOKEN"),
        YookassaSecret: viper.GetString("YOOKASSA_SECRET"),
        R2Endpoint:     viper.GetString("R2_ENDPOINT"),
        R2AccessKey:    viper.GetString("R2_ACCESS_KEY"),
        R2SecretKey:    viper.GetString("R2_SECRET_KEY"),
        R2Bucket:       viper.GetString("R2_BUCKET"),
        JWTSecret:      viper.GetString("JWT_SECRET"),
        CORSOrigins:    corsOrigins,
    }

    // проверка обязательных параметров
    if cfg.DB_DSN == "" {
        log.Printf("Warning: DB_DSN is not set")
    }
    if cfg.TelegramToken == "" {
        log.Printf("Warning: TELEGRAM_BOT_TOKEN is not set")
    }
    if cfg.JWTSecret == "" {
        return nil, fmt.Errorf("JWT_SECRET is required")
    }
    return cfg, nil
}