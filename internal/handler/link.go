package handler

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
)

// Link представляет кнопку или ссылку на лендинге
type Link struct {
    ID        int    `db:"id" json:"id"`
    LandingID int    `db:"landing_id" json:"landingId"`
    Type      string `db:"type" json:"type"`
    Title     string `db:"title" json:"title"`
    URL       string `db:"url" json:"url"`
    Position  int    `db:"position" json:"position"`
}

// RegisterLinkRoutes регистрирует CRUD-эндпоинты для ссылок
func RegisterLinkRoutes(rg *gin.RouterGroup, db *sqlx.DB) {
    r := rg.Group("/links")
    r.GET("", listLinks(db))
    r.POST("", createLink(db))
    r.GET("/:id", getLink(db))
    r.PUT("/:id", updateLink(db))
    r.DELETE("/:id", deleteLink(db))
}

func listLinks(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        landingParam := c.Query("landingId")
        landingID, err := strconv.Atoi(landingParam)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid landingId"})
            return
        }
        var items []Link
        if err := db.Select(&items,
            "SELECT * FROM links WHERE landing_id=$1 ORDER BY position", landingID); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, items)
    }
}

func createLink(db *sqlx.DB) gin.HandlerFunc {
    type request struct {
        LandingID int    `json:"landingId" binding:"required"`
        Type      string `json:"type"      binding:"required"`
        Title     string `json:"title"     binding:"required"`
        URL       string `json:"url"       binding:"required"`
        Position  int    `json:"position"`
    }
    return func(c *gin.Context) {
        var req request
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        var item Link
        query := `
            INSERT INTO links (landing_id, type, title, url, position)
            VALUES ($1,$2,$3,$4,$5)
            RETURNING *`
        if err := db.Get(&item, query,
            req.LandingID, req.Type, req.Title, req.URL, req.Position); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusCreated, item)
    }
}

func getLink(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        id, err := strconv.Atoi(c.Param("id"))
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
            return
        }
        var item Link
        if err := db.Get(&item, "SELECT * FROM links WHERE id=$1", id); err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
            return
        }
        c.JSON(http.StatusOK, item)
    }
}

func updateLink(db *sqlx.DB) gin.HandlerFunc {
    type request struct {
        Type     string `json:"type"     binding:"required"`
        Title    string `json:"title"    binding:"required"`
        URL      string `json:"url"      binding:"required"`
        Position int    `json:"position"`
    }
    return func(c *gin.Context) {
        id, err := strconv.Atoi(c.Param("id"))
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
            return
        }
        var req request
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        var item Link
        query := `
            UPDATE links
               SET type=$1, title=$2, url=$3, position=$4
             WHERE id=$5
          RETURNING *`
        if err := db.Get(&item, query,
            req.Type, req.Title, req.URL, req.Position, id); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, item)
    }
}

func deleteLink(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        id, err := strconv.Atoi(c.Param("id"))
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
            return
        }
        if _, err := db.Exec("DELETE FROM links WHERE id=$1", id); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.Status(http.StatusNoContent)
    }
}