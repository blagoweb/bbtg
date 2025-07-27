// internal/handler/subscription.go
package handler

import (
    "bytes"
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
    "github.com/blagoweb/bbtg/internal/config"
)

// Subscription модель рекуррентного платежа
type Subscription struct {
    ID             int       `db:"id" json:"id"`
    UserID         int       `db:"user_id" json:"userId"`
    PlanID         string    `db:"plan_id" json:"planId"`
    Status         string    `db:"status" json:"status"`
    SubscriptionID string    `db:"subscription_id" json:"subscriptionId"`
    CreatedAt      time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt      time.Time `db:"updated_at" json:"updatedAt"`
}

// RegisterSubscriptionRoutes регистрирует маршруты для подписок
func RegisterSubscriptionRoutes(rg *gin.RouterGroup, db *sqlx.DB, cfg *config.Config) {
    r := rg.Group("/subscriptions")
    r.GET("", listSubscriptions(db))
    r.POST("", createSubscription(db, cfg))
    r.DELETE("/:id", cancelSubscription(db, cfg))
}

// listSubscriptions возвращает все подписки текущего пользователя
func listSubscriptions(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        uidI, exists := c.Get("user_id")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
            return
        }
        uid := int(uidI.(float64))

        var subs []Subscription
        query := `SELECT * FROM subscriptions WHERE user_id=$1 ORDER BY created_at DESC`
        if err := db.Select(&subs, query, uid); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, subs)
    }
}

// createSubscription создаёт рекуррентную подписку через YooKassa и сохраняет в БД
func createSubscription(db *sqlx.DB, cfg *config.Config) gin.HandlerFunc {
    type request struct {
        PlanID string `json:"planId" binding:"required"`
    }
    return func(c *gin.Context) {
        uidI, exists := c.Get("user_id")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
            return
        }
        uid := int(uidI.(float64))

        var req request
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        // Формируем запрос к YooKassa Subscriptions API
        ykURL := "https://payments.yookassa.ru/api/v3/subscriptions"
        payload := map[string]interface{}{
            "plan_id": req.PlanID,
            "metadata": map[string]interface{}{"user_id": uid},
        }
        bodyBytes, _ := json.Marshal(payload)
        httpReq, _ := http.NewRequest("POST", ykURL, bytes.NewReader(bodyBytes))
        httpReq.SetBasicAuth(cfg.YookassaSecret, "")
        httpReq.Header.Set("Content-Type", "application/json")

        client := &http.Client{Timeout: 10 * time.Second}
        resp, err := client.Do(httpReq)
        if err != nil || resp.StatusCode != http.StatusCreated {
            c.JSON(http.StatusBadGateway, gin.H{"error": "yookassa subscription error"})
            return
        }
        defer resp.Body.Close()

        var respData struct {
            ID     string `json:"id"`
            Status string `json:"status"`
        }
        json.NewDecoder(resp.Body).Decode(&respData)

        // Сохраняем подписку в БД
        var sub Subscription
        sql := `INSERT INTO subscriptions (user_id, plan_id, subscription_id, status, created_at, updated_at)
                  VALUES ($1,$2,$3,$4,NOW(),NOW()) RETURNING *`
        if err := db.Get(&sub, sql, uid, req.PlanID, respData.ID, respData.Status); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusCreated, sub)
    }
}

// cancelSubscription отменяет подписку в YooKassa и обновляет статус в БД
func cancelSubscription(db *sqlx.DB, cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        idStr := c.Param("id")
        var sub Subscription
        if err := db.Get(&sub, "SELECT * FROM subscriptions WHERE id=$1", idStr); err != nil {
            if err == sql.ErrNoRows {
                c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
                return
            }
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }

        // Отмена подписки через YooKassa API
        ykURL := fmt.Sprintf("https://payments.yookassa.ru/api/v3/subscriptions/%s/cancel", sub.SubscriptionID)
        httpReq, _ := http.NewRequest("POST", ykURL, nil)
        httpReq.SetBasicAuth(cfg.YookassaSecret, "")

        client := &http.Client{Timeout: 10 * time.Second}
        resp, err := client.Do(httpReq)
        if err != nil || resp.StatusCode != http.StatusOK {
            c.JSON(http.StatusBadGateway, gin.H{"error": "failed to cancel subscription"})
            return
        }
        defer resp.Body.Close()

        // Обновляем статус в БД
        _, err = db.Exec("UPDATE subscriptions SET status='canceled', updated_at=NOW() WHERE id=$1", idStr)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.Status(http.StatusNoContent)
    }
}