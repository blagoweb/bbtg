package payment

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
)

// PaymentEvent описывает тело webhook от YooKassa
type PaymentEvent struct {
    Event struct {
        OperationID string `json:"operation_id"`
        Status      string `json:"status"`
        Amount      struct {
            Value    string `json:"value"`
            Currency string `json:"currency"`
        } `json:"amount"`
        Metadata struct {
            UserID int `json:"user_id"`
        } `json:"metadata"`
    } `json:"event"`
}

// WebhookHandler возвращает обработчик для вебхука YooKassa
func WebhookHandler(db *sqlx.DB, yookassaSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        body, err := io.ReadAll(c.Request.Body)
        if err != nil {
            c.AbortWithStatus(http.StatusBadRequest)
            return
        }
        // Проверяем подпись
        signature := c.GetHeader("X-YaKassa-Signature")
        mac := hmac.New(sha256.New, []byte(yookassaSecret))
        mac.Write(body)
        expected := hex.EncodeToString(mac.Sum(nil))
        if !hmac.Equal([]byte(expected), []byte(signature)) {
            c.AbortWithStatus(http.StatusForbidden)
            return
        }
        // Парсим событие
        var evt PaymentEvent
        if err := json.Unmarshal(body, &evt); err != nil {
            c.AbortWithStatus(http.StatusBadRequest)
            return
        }
        // TODO: Обновить статус платежа в БД по evt.Event.OperationID и evt.Event.Status
        fmt.Printf("Received payment webhook: %v\n", evt)
        c.Status(http.StatusOK)
    }
}