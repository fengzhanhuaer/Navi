// main.go
// Navi 个人导航页 — 单文件可执行入口
// 通过 go:embed 将 frontend/ 打包进二进制，部署只需一个文件

package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"navi/db"
	"navi/handlers"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

// 将 frontend/ 目录嵌入二进制（与 main.go 同级）
//
//go:embed frontend
var embeddedFrontend embed.FS

// Version 由编译时 -ldflags 注入
var Version = "dev"

func main() {
	// 支持健康检查参数（常用于升级新包的预加载验证）
	if len(os.Args) > 1 && os.Args[1] == "--check" {
		fmt.Println("navi binary check passed.")
		os.Exit(0)
	}

	// 加载 .env（不存在时静默忽略）
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] No .env file, using system env")
	}

	log.Printf("[NAVI] Version %s", Version)

	// 初始化本地 SQLite
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	if err := db.Init(dataDir); err != nil {
		log.Fatalf("DB init failed: %v", err)
	}

	// 初始化图标本地缓存目录（与主 DB 分离，不同步到 D1）
	handlers.InitIconCache(dataDir)

	// 初始化 D1 客户端
	d1 := db.NewD1Client()
	if d1.IsConfigured() {
		log.Println("[D1] Cloudflare D1 configured")
		if err := d1.InitD1Tables(); err != nil {
			log.Printf("[D1] WARN: init tables: %v", err)
		}
	} else {
		log.Println("[D1] Not configured — local-only mode")
	}

	// 定时自动同步到 D1
	syncInterval := 5
	if v := os.Getenv("SYNC_INTERVAL_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			syncInterval = n
		}
	}
	if d1.IsConfigured() {
		c := cron.New()
		c.AddFunc(fmt.Sprintf("@every %dm", syncInterval), func() {
			dirty, err := db.GetDirtyData()
			if err != nil || len(dirty.Groups)+len(dirty.Sites)+len(dirty.Settings) == 0 {
				return
			}
			if err := d1.PushDirtyData(dirty); err != nil {
				db.LogSync("error", err.Error())
				return
			}
			db.ClearDirty()
			db.LogSync("ok", fmt.Sprintf("auto sync at %s", time.Now().Format(time.DateTime)))
			log.Printf("[SYNC] OK — groups:%d sites:%d settings:%d",
				len(dirty.Groups), len(dirty.Sites), len(dirty.Settings))
		})
		c.Start()
		log.Printf("[SYNC] Auto sync every %d min", syncInterval)
	}

	// ── Gin ──────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// ── 前端静态文件 ──────────────────────────────
	// FRONTEND_DIR 设置时从磁盘加载（开发热更新）
	// 未设置时从 embed 加载（生产单文件）
	if frontendDir := os.Getenv("FRONTEND_DIR"); frontendDir != "" {
		r.Static("/css", frontendDir+"/css")
		r.Static("/js", frontendDir+"/js")
		r.StaticFile("/", frontendDir+"/index.html")
		r.StaticFile("/favicon.png", frontendDir+"/favicon.png")
		log.Printf("[FRONTEND] Disk: %s", frontendDir)
	} else {
		sub, err := fs.Sub(embeddedFrontend, "frontend")
		if err != nil {
			log.Fatalf("embed: %v", err)
		}
		r.StaticFS("/css", http.FS(mustSubFS(sub, "css")))
		r.StaticFS("/js", http.FS(mustSubFS(sub, "js")))
		r.GET("/", func(c *gin.Context) {
			data, _ := embeddedFrontend.ReadFile("frontend/index.html")
			c.Data(200, "text/html; charset=utf-8", data)
		})
		r.GET("/favicon.png", func(c *gin.Context) {
			data, _ := embeddedFrontend.ReadFile("frontend/favicon.png")
			c.Data(200, "image/png", data)
		})
		log.Println("[FRONTEND] Embedded binary")
	}

	// ── 认证路由（无需登录）─────────────────────
	authGroup := r.Group("/api/auth")
	{
		authGroup.GET("/status", handlers.AuthStatus)
		authGroup.POST("/register", handlers.Register)
		authGroup.POST("/login", handlers.Login)
	}

	// ── 图标代理（无需登录，img 标签直接访问）────
	r.GET("/api/favicon", handlers.GetFavicon)
	r.POST("/api/favicon/refresh", handlers.RefreshFavicon)

	// ── 受保护的 API（需要登录）────────────────
	api := r.Group("/api", handlers.AuthMiddleware())
	{
		api.GET("/data", func(c *gin.Context) {
			groups, _ := db.GetGroups()
			sites, _ := db.GetSites(0)
			settings, _ := db.GetSettings()
			c.JSON(200, gin.H{
				"groups":   groups,
				"sites":    sites,
				"settings": settings,
			})
		})

		api.GET("/groups", handlers.GetGroups)
		api.POST("/groups", handlers.CreateGroup)
		api.PUT("/groups/:id", handlers.UpdateGroup)
		api.DELETE("/groups/:id", handlers.DeleteGroup)
		api.PUT("/groups/reorder", handlers.ReorderGroups)

		api.GET("/sites", handlers.GetSites)
		api.POST("/sites", handlers.CreateSite)
		api.PUT("/sites/:id", handlers.UpdateSite)
		api.DELETE("/sites/:id", handlers.DeleteSite)
		api.PUT("/sites/reorder", handlers.ReorderSites)

		api.GET("/settings", handlers.GetSettings)
		api.PUT("/settings/:key", handlers.SetSetting)
		api.POST("/settings/upgrade", handlers.UpgradeSystem)

		api.POST("/sync/push", handlers.SyncToD1(d1))
		api.POST("/sync/restore", handlers.RestoreFromD1(d1))
		api.GET("/sync/status", handlers.GetD1Status(d1))
		api.GET("/sync/logs", handlers.GetSyncLogs)

		api.POST("/d1/configure", handlers.ConfigureD1(d1))
	}

	// ── 独立页面路由 ──────────────────────
	if frontendDir2 := os.Getenv("FRONTEND_DIR"); frontendDir2 != "" {
		r.StaticFile("/settings", frontendDir2+"/settings.html")
		r.StaticFile("/login", frontendDir2+"/login.html")
		r.StaticFile("/register", frontendDir2+"/register.html")
		r.StaticFile("/edit", frontendDir2+"/edit.html")
	} else {
		r.GET("/settings", func(c *gin.Context) {
			data, _ := embeddedFrontend.ReadFile("frontend/settings.html")
			c.Data(200, "text/html; charset=utf-8", data)
		})
		r.GET("/login", func(c *gin.Context) {
			data, _ := embeddedFrontend.ReadFile("frontend/login.html")
			c.Data(200, "text/html; charset=utf-8", data)
		})
		r.GET("/register", func(c *gin.Context) {
			data, _ := embeddedFrontend.ReadFile("frontend/register.html")
			c.Data(200, "text/html; charset=utf-8", data)
		})
		r.GET("/system-settings", func(c *gin.Context) {
			data, _ := embeddedFrontend.ReadFile("frontend/system-settings.html")
			c.Data(200, "text/html; charset=utf-8", data)
		})
		r.GET("/edit", func(c *gin.Context) {
			data, _ := embeddedFrontend.ReadFile("frontend/edit.html")
			c.Data(200, "text/html; charset=utf-8", data)
		})
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": Version, "time": time.Now().Format(time.RFC3339)})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "15020"
	}
	log.Printf("[NAVI] Listening on http://0.0.0.0:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// mustSubFS 返回 embed FS 的子目录，出错则 panic
func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		log.Fatalf("embed subfs %q: %v", dir, err)
	}
	return sub
}
