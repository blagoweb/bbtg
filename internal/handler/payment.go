// internal/handler/payment.go
package handler

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
)

// Payment представляет запись о платеже
type Payment struct {
    ID            int     `db:"id" json:"id"`
    UserID        int     `db:"user_id" json:"userId"`
    Amount        string  `db:"amount" json:"amount"`
    Currency      string  `db:"currency" json:"currency"`
    PaymentMethod string  `db:"payment_method" json:"paymentMethod"`
    Status        string  `db:"status" json:"status"`
    TransactionID string  `db:"transaction_id" json:"transactionId"`
    CreatedAt     string  `db:"created_at" json:"createdAt"`
    UpdatedAt     string  `db:"updated_at" json:"updatedAt"`
}

// RegisterPaymentRoutes регистрирует маршруты для платежей
func RegisterPaymentRoutes(rg *gin.RouterGroup, db *sqlx.DB) {
    r := rg.Group("/payments")
    r.GET("", listPayments(db))
    r.GET("/:id", getPayment(db))
}

// listPayments возвращает список платежей текущего пользователя
func listPayments(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        uidI, exists := c.Get("user_id")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
            return
        }
        uid, err := strconv.Atoi(uidI.(string))
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id"})
            return
        }

        var items []Payment
        if err := db.Select(&items,
            "SELECT * FROM payments WHERE user_id=$1 ORDER BY created_at DESC", uid); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, items)
    }
}

// getPayment возвращает платёж по ID, если он принадлежит текущему пользователю
func getPayment(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        uidI, exists := c.Get("user_id")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
            return
        }
        uid, err := strconv.Atoi(uidI.(string))
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id"})
            return
        }

        idParam := c.Param("id")
        id, err := strconv.Atoi(idParam)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
            return
        }

        var p Payment
        query := `SELECT * FROM payments WHERE id=$1 AND user_id=$2`
        if err := db.Get(&p, query, id, uid); err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
            return
        }
        c.JSON(http.StatusOK, p)
    }
}
