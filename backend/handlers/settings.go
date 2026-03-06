// handlers/settings.go
// 配置 & D1 同步相关处理器

package handlers

import (
	"net/http"

	"navi/db"

	"github.com/gin-gonic/gin"
)

func GetSettings(c *gin.Context) {
	settings, err := db.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func SetSetting(c *gin.Context) {
	key := c.Param("key")
	var body struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := db.SetSetting(key, body.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SyncToD1 手动触发同步到 D1
func SyncToD1(d1 *db.D1HTTPClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		dirty, err := db.GetDirtyData()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := d1.PushDirtyData(dirty); err != nil {
			db.LogSync("error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		db.ClearDirty()
		db.LogSync("ok", "manual sync")
		c.JSON(http.StatusOK, gin.H{"ok": true, "synced": map[string]int{
			"groups":   len(dirty.Groups),
			"sites":    len(dirty.Sites),
			"settings": len(dirty.Settings),
		}})
	}
}

// RestoreFromD1 从 D1 拉取数据覆盖本地（慎用）
func RestoreFromD1(d1 *db.D1HTTPClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := d1.PullAll()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := db.RestoreFromD1Data(data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		db.LogSync("ok", "restore from D1")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetD1Status 检查 D1 连接状态
func GetD1Status(d1 *db.D1HTTPClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		status, err := d1.GetD1Status()
		resp := gin.H{"status": status}
		if err != nil {
			resp["error"] = err.Error()
		}
		c.JSON(http.StatusOK, resp)
	}
}

// GetSyncLogs 查看同步日志
func GetSyncLogs(c *gin.Context) {
	logs, err := db.GetSyncLogs(50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if logs == nil {
		logs = []map[string]string{}
	}
	c.JSON(http.StatusOK, logs)
}
