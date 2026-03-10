// handlers/favicon.go
// 站点图标本地缓存层
//
// 抓取策略（按序降级）：
//   1. DuckDuckGo 图标服务（国内可达，速度快）
//   2. 解析站点 HTML，找 <link rel="icon"> 指定的真实路径
//   3. 直接尝试 /favicon.ico、/favicon.png
//   4. Google favicon 服务（国外备用）

package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var iconCacheDir string

// InitIconCache 设置图标缓存目录
func InitIconCache(dataDir string) {
	iconCacheDir = filepath.Join(dataDir, "icons")
	if err := os.MkdirAll(iconCacheDir, 0755); err != nil {
		log.Printf("[ICON] WARN: cannot create icon cache dir: %v", err)
	} else {
		log.Printf("[ICON] Cache dir: %s", iconCacheDir)
	}
}

func domainKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		u2, err2 := url.Parse("https://" + rawURL)
		if err2 != nil || u2.Host == "" {
			r := strings.NewReplacer("/", "_", ":", "_", "?", "_", "&", "_", "=", "_")
			return r.Replace(rawURL)
		}
		u = u2
	}
	return strings.ToLower(u.Hostname())
}

func cachePath(key string) string {
	return filepath.Join(iconCacheDir, key+".png")
}

// newBrowserClient 创建使用真实浏览器 UA 的 HTTP 客户端
func newBrowserClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// browserUA 模拟真实 Chrome 浏览器，绕过简单 Bot 检测
const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

// ── GET /api/favicon?url=<site_url> ──────────────────────
func GetFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	path := cachePath(domainKey(siteURL))
	if _, err := os.Stat(path); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	// 使用 no-cache：浏览器每次都发条件请求验证，确保远端更新图标后能及时呈现
	c.Header("Cache-Control", "no-cache")
	c.File(path)
}

// ── POST /api/favicon/fetch?url=<site_url> ───────────────
func FetchAndCacheFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}

	u, err := url.Parse(siteURL)
	if err != nil || u.Host == "" {
		u, err = url.Parse("https://" + siteURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
			return
		}
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	key := domainKey(siteURL)
	path := cachePath(key)
	origin := u.Scheme + "://" + u.Host
	client := newBrowserClient(10 * time.Second)

	// ── 第一优先：解析原始站点 HTML 找 <link rel="icon"> ──
	// 直接访问源站，获取网站最新更新的图标
	if iconURL := extractFaviconFromHTML(client, u); iconURL != "" {
		if data, err := fetchImage(client, iconURL); err == nil && len(data) >= 50 {
			if saveAndRespond(c, path, key, data, iconURL) {
				return
			}
		}
	}

	// ── 第二优先：直接尝试标准路径 ──────────────────────
	for _, suffix := range []string{"/favicon.ico", "/favicon.png", "/apple-touch-icon.png"} {
		src := origin + suffix
		if data, err := fetchImage(client, src); err == nil && len(data) >= 50 {
			if saveAndRespond(c, path, key, data, src) {
				return
			}
		}
	}

	// ── 第三优先：DuckDuckGo（可能有缓存延迟，降为第三）──
	ddgURL := fmt.Sprintf("https://icons.duckduckgo.com/ip3/%s.ico", u.Hostname())
	if data, err := fetchImage(client, ddgURL); err == nil && len(data) >= 50 {
		if saveAndRespond(c, path, key, data, ddgURL) {
			return
		}
	}

	// ── 兜底：Google favicon 服务（国外备用）────────────
	googleURL := fmt.Sprintf("https://www.google.com/s2/favicons?domain=%s&sz=64", u.Hostname())
	if data, err := fetchImage(client, googleURL); err == nil && len(data) >= 50 {
		if saveAndRespond(c, path, key, data, googleURL) {
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": false, "error": "no favicon found for " + u.Hostname()})
}

// extractFaviconFromHTML 获取页面 HTML，用正则提取 <link rel="icon"> 的 href
func extractFaviconFromHTML(client *http.Client, u *url.URL) string {
	pageURL := u.Scheme + "://" + u.Host + "/"
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")

	// 使用较短超时，避免阻塞太久
	htmlClient := newBrowserClient(6 * time.Second)
	resp, err := htmlClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// 只读前 64KB（favicon link 通常在 <head> 内）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return ""
	}
	html := string(body)

	// 匹配 <link rel="icon" ...> / <link rel="shortcut icon" ...> / <link rel="apple-touch-icon" ...>
	// 支持属性乱序、单双引号
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)<link[^>]+rel=["'](?:shortcut )?icon["'][^>]+href=["']([^"']+)["']`),
		regexp.MustCompile(`(?i)<link[^>]+href=["']([^"']+)["'][^>]+rel=["'](?:shortcut )?icon["']`),
		regexp.MustCompile(`(?i)<link[^>]+rel=["']apple-touch-icon["'][^>]+href=["']([^"']+)["']`),
		regexp.MustCompile(`(?i)<link[^>]+href=["']([^"']+)["'][^>]+rel=["']apple-touch-icon["']`),
	}

	for _, re := range patterns {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			href := strings.TrimSpace(m[1])
			if href == "" || strings.HasPrefix(href, "data:") {
				continue
			}
			// 转为绝对 URL
			if strings.HasPrefix(href, "//") {
				return u.Scheme + ":" + href
			}
			if strings.HasPrefix(href, "/") {
				return u.Scheme + "://" + u.Host + href
			}
			if strings.HasPrefix(href, "http") {
				return href
			}
			// 相对路径
			return u.Scheme + "://" + u.Host + "/" + href
		}
	}
	return ""
}

func saveAndRespond(c *gin.Context, path, key string, data []byte, src string) bool {
	if err := os.WriteFile(path, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write cache: " + err.Error()})
		return true
	}
	log.Printf("[ICON] Cached: %s via %s (%d bytes)", key, src, len(data))
	c.JSON(http.StatusOK, gin.H{"ok": true, "source": src})
	return true
}

func fetchImage(client *http.Client, src string) ([]byte, error) {
	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	// 放宽 Content-Type 检查（有些站返回 application/octet-stream 或无类型）
	if ct != "" && strings.Contains(ct, "text/html") {
		return nil, fmt.Errorf("html response, not image")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}
	// 简单检查魔数：PNG / ICO / GIF / JPEG / WebP / SVG
	if len(data) > 4 && !isImageData(data) {
		return nil, fmt.Errorf("not image data")
	}
	return data, nil
}

func isImageData(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	// PNG: 89 50 4E 47
	if b[0] == 0x89 && b[1] == 0x50 {
		return true
	}
	// JPEG: FF D8
	if b[0] == 0xFF && b[1] == 0xD8 {
		return true
	}
	// GIF: 47 49 46
	if b[0] == 0x47 && b[1] == 0x49 && b[2] == 0x46 {
		return true
	}
	// ICO: 00 00 01 00
	if b[0] == 0x00 && b[1] == 0x00 && b[2] == 0x01 && b[3] == 0x00 {
		return true
	}
	// WebP: 52 49 46 46 ... 57 45 42 50
	if b[0] == 0x52 && b[1] == 0x49 {
		return true
	}
	// SVG: starts with <svg or <?xml
	s := strings.ToLower(strings.TrimSpace(string(b[:min(50, len(b))])))
	return strings.HasPrefix(s, "<svg") || strings.HasPrefix(s, "<?xml")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── POST /api/favicon/upload ─────────────────────────────
func UploadFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, 512*1024))
	if err != nil || len(data) < 16 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image data"})
		return
	}
	path := cachePath(domainKey(siteURL))
	if err := os.WriteFile(path, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── DELETE /api/favicon ──────────────────────────────────
func DeleteFavicon(c *gin.Context) {
	siteURL := strings.TrimSpace(c.Query("url"))
	if siteURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}
	os.Remove(cachePath(domainKey(siteURL)))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
