package handler

import (
    "fmt"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
    "github.com/blagoweb/bbtg/internal/telegram"
)

// Lead представляет заявку пользователя
type Lead struct {
    ID        int    `db:"id" json:"id"`
    LandingID int    `db:"landing_id" json:"landingId"`
    Name      string `db:"name" json:"name"`
    Email     string `db:"email" json:"email"`
    Phone     string `db:"phone" json:"phone"`
    Message   string `db:"message" json:"message"`
}

// RegisterLeadRoutes регистрирует маршруты для работы с лидами
func RegisterLeadRoutes(rg *gin.RouterGroup, db *sqlx.DB, bot *telegram.Bot) {
    r := rg.Group("/leads")
    r.GET("", listLeads(db))
    r.POST("", createLead(db, bot))
}

// listLeads возвращает все лиды всех лендингов текущего пользователя
func listLeads(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        // получаем user_id из контекста
        uidI, exists := c.Get("user_id")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
            return
        }
        uid, err := strconv.Atoi(fmt.Sprint(uidI))
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id"})
            return
        }
        // выбираем лиды по всем лендингам пользователя
        query := `SELECT l.* FROM leads l
                  JOIN landings g ON g.id = l.landing_id
                  WHERE g.user_id = $1
                  ORDER BY l.created_at DESC`
        var items []Lead
        if err := db.Select(&items, query, uid); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, items)
    }
}

// createLead создаёт новый лид и отправляет уведомление в Telegram
func createLead(db *sqlx.DB, bot *telegram.Bot) gin.HandlerFunc {
    type request struct {
        LandingID int    `json:"landingId" binding:"required"`
        Name      string `json:"name"`
        Email     string `json:"email"`
        Phone     string `json:"phone"`
        Message   string `json:"message"`
    }
    return func(c *gin.Context) {
        var req request
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        // сохраняем в БД
        var lead Lead
        sql := `INSERT INTO leads (landing_id, name, email, phone, message)
                VALUES ($1,$2,$3,$4,$5)
                RETURNING id, landing_id, name, email, phone, message`
        if err := db.Get(&lead, sql,
            req.LandingID, req.Name, req.Email, req.Phone, req.Message); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        // отправляем уведомление ботом
        text := fmt.Sprintf("Новая заявка:\nЛендинг: %d\nИмя: %s\nEmail: %s\nТелефон: %s\nСообщение: %s",
            lead.LandingID, lead.Name, lead.Email, lead.Phone, lead.Message)
        if err := bot.SendNotification(text); err != nil {
            // логируем, но не мешаем пользователю
            fmt.Printf("bot send error: %v", err)
        }
        c.JSON(http.StatusCreated, lead)
    }
}