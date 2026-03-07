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

// GetD1Status 检查 D1 连接状态，同时返回已配置的账户信息
func GetD1Status(d1 *db.D1HTTPClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		status, err := d1.GetD1Status()
		accountID, _ := db.GetSettingPublic("cf_account_id")
		databaseID, _ := db.GetSettingPublic("cf_database_id")
		resp := gin.H{
			"status":      status,
			"account_id":  accountID,
			"database_id": databaseID,
		}
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

// ConfigureD1 接收 api_token，自动发现账户 + 查找/创建固定名称 "navi" 的数据库
func ConfigureD1(d1 *db.D1HTTPClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			APIToken string `json:"api_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 api_token"})
			return
		}
		if err := db.ConfigureWithAPIKey(body.APIToken, "navi"); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// 重新加载配置，初始化 D1 表
		if err := d1.InitD1Tables(); err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": true, "warn": "配置成功，但初始化表失败: " + err.Error()})
			return
		}
		db.LogSync("ok", "D1 configured via API key")
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "配置成功，已连接到 D1 数据库 navi"})
	}
}

