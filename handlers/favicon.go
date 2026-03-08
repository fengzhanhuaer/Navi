// handlers/favicon.go
// 站点图标代理 & 本地缓存层
// 策略：
//   1. 按域名在 data/icons/<domain>.png 缓存
//   2. 优先从目标站 /favicon.ico 抓取
//   3. 失败则回退到 Google favicon 服务
//   4. 再失败返回内置默认图标（1x1 透明 PNG）

package handlers

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// domainKey 将 URL 转为安全的文件名 key（md5 of origin）
func domainKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		// 尝试加协议头再解析
		u2, err2 := url.Parse("https://" + rawURL)
		if err2 != nil || u2.Host == "" {
			// fallback: md5 of raw string
			return fmt.Sprintf("%x", md5.Sum([]byte(rawURL)))
		}
		u = u2
	}
	host := strings.ToLower(u.Hostname())
	return host
}

// cachePath 返回缓存文件完整路径
func cachePath(key string) string {
	return filepath.Join(iconCacheDir, key+".png")
}

// fetchAndCache 抓取图标并写入缓存，返回文件路径
func fetchAndCache(siteURL string) (string, error) {
	key := domainKey(siteURL)
	path := cachePath(key)

	// 已缓存则直接返回
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// 解析 origin
	u, err := url.Parse(siteURL)
	if err != nil || u.Host == "" {
		u, err = url.Parse("https://" + siteURL)
		if err != nil {
			return "", fmt.Errorf("invalid url: %w", err)
		}
	}
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	if u.Scheme == "" {
		origin = "https://" + u.Host
	}

	client := &http.Client{Timeout: 8 * time.Second}

	// 候选 favicon URL 列表
	candidates := []string{
		origin + "/favicon.ico",
		origin + "/favicon.png",
		fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=64", u.Hostname()),
	}

	var imgData []byte
	for _, candidate := range candidates {
		data, err := fetchURL(client, candidate)
		if err != nil {
			continue
		}
		// 检查是否是有效图片（至少 50 字节，不是 html）
		if len(data) > 50 && !isHTMLResponse(data) {
			imgData = data
			break
		}
	}

	if imgData == nil {
		return "", fmt.Errorf("no favicon found for %s", siteURL)
	}

	// 写入缓存
	if err := os.WriteFile(path, imgData, 0644); err != nil {
		return "", fmt.Errorf("write cache: %w", err)
	}
	log.Printf("[ICON] Cached: %s → %s", u.Hostname(), key+".png")
	return path, nil
}

func fetchURL(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Navi/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // 限制 512KB
}

func isHTMLResponse(data []byte) bool {
	s := strings.ToLower(strings.TrimSpace(string(data[:min(len(data), 100)])))
	return strings.HasPrefix(s, "<!doctype") || strings.HasPrefix(s, "<html")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// defaultIcon 返回一个简单的默认图标（16x16 灰色方块 ICO）
// 实际上我们直接返回 404 让前端 fallback
var defaultPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG header
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, // IDAT chunk (1x1 gray pixel)
	0x54, 0x08, 0xd7, 0x63, 0xa8, 0xa8, 0xa8, 0x00,
	0x00, 0x00, 0x03, 0x00, 0x01, 0x6d, 0x2d, 0xcb,
	0xb2, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, // IEND chunk
	0x44, 0xae, 0x42, 0x60, 0x82,
}

// ── HTTP Handler ──────────────────────────────────────────

// GetFavicon 处理 GET /api/favicon?url=<site_url>
// 返回图标文件（image/png），先从缓存，未命中则抓取并缓存
func GetFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.Data(http.StatusOK, "image/png", defaultPNG)
		return
	}

	// 尝试从缓存获取
	key := domainKey(siteURL)
	path := cachePath(key)

	if _, err := os.Stat(path); err == nil {
		// 缓存命中，直接提供
		c.Header("Cache-Control", "public, max-age=86400")
		c.File(path)
		return
	}

	// 缓存未命中，异步抓取
	path, err := fetchAndCache(siteURL)
	if err != nil {
		log.Printf("[ICON] Fetch failed for %s: %v", siteURL, err)
		// 返回 Google favicon 作为最终回退
		u, _ := url.Parse(siteURL)
		if u == nil {
			u, _ = url.Parse("https://" + siteURL)
		}
		if u != nil && u.Host != "" {
			c.Redirect(http.StatusFound, fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=64", u.Hostname()))
			return
		}
		c.Data(http.StatusOK, "image/png", defaultPNG)
		return
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.File(path)
}

// RefreshFavicon 处理 POST /api/favicon/refresh?url=<site_url>
// 强制重新抓取（删除缓存后重新获取）
func RefreshFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}

	// 删除旧缓存
	key := domainKey(siteURL)
	path := cachePath(key)
	os.Remove(path)

	// 重新抓取
	if _, err := fetchAndCache(siteURL); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
