// handlers/favicon.go
// 站点图标本地缓存层（只存不抓）
// 图标由前端采集后 POST 上传，服务端负责存储和提供。

package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// iconCacheDir 由 InitIconCache 初始化
var iconCacheDir string

// InitIconCache 设置图标缓存目录（在 main.go 中调用）
func InitIconCache(dataDir string) {
	iconCacheDir = filepath.Join(dataDir, "icons")
	if err := os.MkdirAll(iconCacheDir, 0755); err != nil {
		log.Printf("[ICON] WARN: cannot create icon cache dir: %v", err)
	} else {
		log.Printf("[ICON] Cache dir: %s", iconCacheDir)
	}
}

// domainKey 将 URL 转为安全的文件名（域名本身即可，已足够唯一）
func domainKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		u2, err2 := url.Parse("https://" + rawURL)
		if err2 != nil || u2.Host == "" {
			// fallback: 用 rawURL 的简单清洗
			r := strings.NewReplacer("/", "_", ":", "_", "?", "_", "&", "_", "=", "_")
			return r.Replace(rawURL)
		}
		u = u2
	}
	return strings.ToLower(u.Hostname())
}

// cachePath 返回缓存文件完整路径
func cachePath(key string) string {
	return filepath.Join(iconCacheDir, key+".png")
}

// ── HTTP Handlers ─────────────────────────────────────────

// GetFavicon 处理 GET /api/favicon?url=<site_url>
// 只从本地缓存提供，未缓存则返回 404（前端应先调用 Upload）
func GetFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	key := domainKey(siteURL)
	path := cachePath(key)

	if _, err := os.Stat(path); err != nil {
		// 未缓存：返回 404，前端负责降级处理
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Cache-Control", "public, max-age=604800") // 缓存 7 天
	c.File(path)
}

// UploadFavicon 处理 POST /api/favicon/upload?url=<site_url>
// 前端将 favicon 二进制数据上传，服务端存入本地缓存
// Content-Type: image/* 或 application/octet-stream
// Body: 原始图片字节（PNG/ICO/WebP 均可，存为 .png 提供）
func UploadFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}

	// 读取请求体（最多 512KB）
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, 512*1024))
	if err != nil || len(data) < 16 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or empty image data"})
		return
	}

	key := domainKey(siteURL)
	path := cachePath(key)

	if err := os.WriteFile(path, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("write cache: %v", err)})
		return
	}

	log.Printf("[ICON] Stored: %s → %s.png (%d bytes)", siteURL, key, len(data))
	c.JSON(http.StatusOK, gin.H{"ok": true, "key": key})
}

// DeleteFavicon 处理 DELETE /api/favicon?url=<site_url>
// 删除本地缓存的图标（网站删除时可选调用）
func DeleteFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}
	key := domainKey(siteURL)
	path := cachePath(key)
	os.Remove(path) // 不存在也不报错
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
