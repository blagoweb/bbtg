package handler

import (
    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
    "net/http"
    "strconv"
)

// AnalyticsEvent представляет запись аналитики
type AnalyticsEvent struct {
    ID        int    `db:"id" json:"id"`
    LandingID int    `db:"landing_id" json:"landingId"`
    EventType string `db:"event_type" json:"eventType"`
    GeoCountry string `db:"geo_country" json:"geoCountry"`
    GeoCity    string `db:"geo_city" json:"geoCity"`
    IPAddress  string `db:"ip_address" json:"ipAddress"`
    UserAgent  string `db:"user_agent" json:"userAgent"`
    CreatedAt  string `db:"created_at" json:"createdAt"`
}

// RegisterAnalyticsRoutes регистрирует маршруты для аналитики
func RegisterAnalyticsRoutes(rg *gin.RouterGroup, db *sqlx.DB) {
    r := rg.Group("/analytics")
    r.GET("", listAnalytics(db))
    r.POST("", createAnalytics(db))
}

// listAnalytics возвращает события аналитики для конкретного лендинга
func listAnalytics(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        landingParam := c.Query("landingId")
        landingID, err := strconv.Atoi(landingParam)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid landingId"})
            return
        }
        var items []AnalyticsEvent
        query := `SELECT * FROM analytics WHERE landing_id=$1 ORDER BY created_at DESC`
        if err := db.Select(&items, query, landingID); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, items)
    }
}

// createAnalytics сохраняет новое событие аналитики
func createAnalytics(db *sqlx.DB) gin.HandlerFunc {
    type request struct {
        LandingID int    `json:"landingId" binding:"required"`
        EventType string `json:"eventType" binding:"required"`
        GeoCountry string `json:"geoCountry"`
        GeoCity    string `json:"geoCity"`
        IPAddress  string `json:"ipAddress"`
        UserAgent  string `json:"userAgent"`
    }
    return func(c *gin.Context) {
        var req request
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        var evt AnalyticsEvent
        query := `INSERT INTO analytics (landing_id, event_type, geo_country, geo_city, ip_address, user_agent)
                  VALUES ($1,$2,$3,$4,$5,$6) RETURNING *`
        if err := db.Get(&evt, query,
            req.LandingID, req.EventType, req.GeoCountry, req.GeoCity, req.IPAddress, req.UserAgent); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusCreated, evt)
    }
}