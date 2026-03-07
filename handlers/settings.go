// handlers/settings.go
// 配置 & D1 同步相关处理器

package handlers

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"navi/db"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v59/github"
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

// UpgradeSystem 从 GitHub 拉取最新 release 并覆盖本地二进制文件
func UpgradeSystem(c *gin.Context) {
	// 由于这只是个个人项目示例，此处硬编码仓库所有者和名称。
	// 实际开发中应该让用户或系统变量配置。
	repoOwner := "fengzhanhuaer"
	repoName := "Navi"
	
	client := github.NewClient(nil)
	ctx := context.Background()
	
	release, _, err := client.Repositories.GetLatestRelease(ctx, repoOwner, repoName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取最新版本失败: " + err.Error()})
		return
	}

	// 查找匹配当前系统架构的资产文件
	var assetURL string
	var assetName string
	targetSuffix := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH) // ex: windows_amd64
	
	for _, asset := range release.Assets {
		if strings.Contains(*asset.Name, targetSuffix) {
			assetURL = *asset.BrowserDownloadURL
			assetName = *asset.Name
			break
		}
	}

	if assetURL == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "没有找到适合当前系统平台或架构的发布资源包"})
		return
	}

	// 下载 Release 压缩包
	resp, err := http.Get(assetURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "下载失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	tmpDir, err := os.MkdirTemp("", "navi-update-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时目录失败: " + err.Error()})
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpZipPath := filepath.Join(tmpDir, assetName)
	out, err := os.Create(tmpZipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时文件失败: " + err.Error()})
		return
	}
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存压缩包失败: " + err.Error()})
		return
	}

	// 在 Windows 上我们通常找 .zip，解压之
	if strings.HasSuffix(assetName, ".zip") {
		r, err := zip.OpenReader(tmpZipPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "解压失败: " + err.Error()})
			return
		}
		defer r.Close()

		var binFile *zip.File
		for _, f := range r.File {
			if strings.HasSuffix(f.Name, "navi.exe") || strings.HasSuffix(f.Name, "navi") {
				binFile = f
				break
			}
		}

		if binFile == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩包中未找到 navi 执行文件"})
			return
		}

		rc, err := binFile.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取执行文件失败: " + err.Error()})
			return
		}
		defer rc.Close()

		currExec, err := os.Executable()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取当前执行路径失败: " + err.Error()})
			return
		}

		// Windows 下正在运行的程序可能无法被直接覆盖，通常的做法是重命名旧文件
		oldExec := currExec + ".old"
		os.Remove(oldExec) // 忽略错误
		if err := os.Rename(currExec, oldExec); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "重命名旧文件失败: " + err.Error()})
			return
		}

		newExec, err := os.OpenFile(currExec, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
		if err != nil {
			// 如果写入新文件失败，至少尝试还原旧文件
			os.Rename(oldExec, currExec)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "提取新执行文件失败: " + err.Error()})
			return
		}
		_, err = io.Copy(newExec, rc)
		newExec.Close()
		if err != nil {
			os.Rename(oldExec, currExec)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "写入新执行文件失败: " + err.Error()})
			return
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "不支持的打包格式，预期为 .zip"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "log": fmt.Sprintf("从版本 %s 成功升级到 %s，请重启服务。", "当前", *release.TagName)})
}

