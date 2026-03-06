// handlers/sites.go
// 网站相关 HTTP 处理器

package handlers

import (
	"net/http"
	"strconv"

	"navi/db"

	"github.com/gin-gonic/gin"
)

func GetSites(c *gin.Context) {
	var groupID int64
	if gid := c.Query("group_id"); gid != "" {
		groupID, _ = strconv.ParseInt(gid, 10, 64)
	}
	sites, err := db.GetSites(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if sites == nil {
		sites = []db.Site{}
	}
	c.JSON(http.StatusOK, sites)
}

func CreateSite(c *gin.Context) {
	var body struct {
		GroupID int64  `json:"group_id" binding:"required"`
		Title   string `json:"title" binding:"required"`
		URL     string `json:"url" binding:"required"`
		Icon    string `json:"icon"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, err := db.CreateSite(body.GroupID, body.Title, body.URL, body.Icon)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func UpdateSite(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		GroupID int64  `json:"group_id" binding:"required"`
		Title   string `json:"title" binding:"required"`
		URL     string `json:"url" binding:"required"`
		Icon    string `json:"icon"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := db.UpdateSite(id, body.GroupID, body.Title, body.URL, body.Icon); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteSite(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := db.DeleteSite(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func ReorderSites(c *gin.Context) {
	var body []struct {
		ID    int64 `json:"id"`
		Order int   `json:"order"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items := make([]struct{ ID int64; Order int }, len(body))
	for i, v := range body {
		items[i] = struct{ ID int64; Order int }{v.ID, v.Order}
	}
	if err := db.ReorderSites(items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
