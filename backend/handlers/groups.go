// handlers/groups.go
// 分组相关 HTTP 处理器

package handlers

import (
	"net/http"
	"strconv"

	"navi/db"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groups, err := db.GetGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if groups == nil {
		groups = []db.SiteGroup{}
	}
	c.JSON(http.StatusOK, groups)
}

func CreateGroup(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
		Icon string `json:"icon"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Icon == "" {
		body.Icon = "📁"
	}
	id, err := db.CreateGroup(body.Name, body.Icon)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func UpdateGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Name      string `json:"name" binding:"required"`
		Icon      string `json:"icon"`
		Collapsed bool   `json:"collapsed"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := db.UpdateGroup(id, body.Name, body.Icon, body.Collapsed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := db.DeleteGroup(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func ReorderGroups(c *gin.Context) {
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
	if err := db.ReorderGroups(items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
