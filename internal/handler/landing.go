package handler

import (
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
    "github.com/blagoweb/bbtg/internal/storage/r2"
)

// Landing представляет лендинг-страницу пользователя
type Landing struct {
    ID          int       `db:"id" json:"id"`
    UserID      int       `db:"user_id" json:"userId"`
    Title       string    `db:"title" json:"title"`
    Description string    `db:"description" json:"description"`
    AvatarURL   string    `db:"avatar_url" json:"avatarUrl"`
    CreatedAt   time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

// RegisterLandingRoutes регистрирует CRUD-эндпоинты для лендингов
func RegisterLandingRoutes(rg *gin.RouterGroup, db *sqlx.DB, _ *r2.Client) {
    r := rg.Group("/landings")
    r.GET("", listLandings(db))
    r.POST("", createLanding(db))
    r.GET(":id", getLanding(db))
    r.PUT(":id", updateLanding(db))
    r.DELETE(":id", deleteLanding(db))
}

// listLandings возвращает все лендинги пользователя
func listLandings(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        telegramIDI, exists := c.Get("telegram_id")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "telegram_id not found"})
            return
        }
        telegramID, err := strconv.ParseInt(fmt.Sprint(telegramIDI), 10, 64)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram_id"})
            return
        }

        // Сначала находим пользователя по telegram_id
        var userID int
        if err := db.Get(&userID, "SELECT id FROM users WHERE telegram_id=$1", telegramID); err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
            return
        }

        var items []Landing
        if err := db.Select(&items, "SELECT * FROM landings WHERE user_id=$1 ORDER BY created_at DESC", userID); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, items)
    }
}

// createLanding создаёт новый лендинг
func createLanding(db *sqlx.DB) gin.HandlerFunc {
    type request struct {
        Title       string `json:"title" binding:"required"`
        Description string `json:"description"`
        AvatarURL   string `json:"avatarUrl"`
    }
    return func(c *gin.Context) {
        telegramIDI, exists := c.Get("telegram_id")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "telegram_id not found"})
            return
        }
        telegramID, err := strconv.ParseInt(fmt.Sprint(telegramIDI), 10, 64)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram_id"})
            return
        }

        // Сначала находим пользователя по telegram_id
        var userID int
        if err := db.Get(&userID, "SELECT id FROM users WHERE telegram_id=$1", telegramID); err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
            return
        }

        var req request
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        var item Landing
        query := `INSERT INTO landings(user_id, title, description, avatar_url, created_at, updated_at)
                  VALUES($1,$2,$3,$4,NOW(),NOW()) RETURNING *`
        if err := db.Get(&item, query, userID, req.Title, req.Description, req.AvatarURL); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusCreated, item)
    }
}

// getLanding возвращает лендинг по ID
func getLanding(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        idParam := c.Param("id")
        id, err := strconv.Atoi(idParam)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
            return
        }

        var item Landing
        if err := db.Get(&item, "SELECT * FROM landings WHERE id=$1", id); err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
            return
        }
        c.JSON(http.StatusOK, item)
    }
}

// updateLanding обновляет существующий лендинг
func updateLanding(db *sqlx.DB) gin.HandlerFunc {
    type request struct {
        Title       string `json:"title" binding:"required"`
        Description string `json:"description"`
        AvatarURL   string `json:"avatarUrl"`
    }
    return func(c *gin.Context) {
        idParam := c.Param("id")
        id, err := strconv.Atoi(idParam)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
            return
        }

        var req request
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        query := `UPDATE landings SET title=$1, description=$2, avatar_url=$3, updated_at=NOW() WHERE id=$4 RETURNING *`
        var item Landing
        if err := db.Get(&item, query, req.Title, req.Description, req.AvatarURL, id); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, item)
    }
}

// deleteLanding удаляет лендинг по ID
func deleteLanding(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        idParam := c.Param("id")
        id, err := strconv.Atoi(idParam)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
            return
        }

        if _, err := db.Exec("DELETE FROM landings WHERE id=$1", id); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.Status(http.StatusNoContent)
    }
}