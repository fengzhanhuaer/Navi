// main.go
// Navi 个人导航页后端入口
// 启动 HTTP 服务器，注册路由，启动定时 D1 同步

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"navi/db"
	"navi/handlers"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

func main() {
	// 加载 .env
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] No .env file found, using system env")
	}

	// 初始化本地 SQLite
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	frontendDir := os.Getenv("FRONTEND_DIR")
	if frontendDir == "" {
		frontendDir = "../frontend"
	}
	if err := db.Init(dataDir); err != nil {
		log.Fatalf("DB init failed: %v", err)
	}

	// 初始化 D1 客户端
	d1 := db.NewD1Client()
	if d1.IsConfigured() {
		log.Println("[D1] Cloudflare D1 configured")
		// 确保 D1 表存在
		if err := d1.InitD1Tables(); err != nil {
			log.Printf("[D1] WARN: init tables: %v", err)
		}
	} else {
		log.Println("[D1] Not configured — running in local-only mode")
	}

	// 启动定时自动同步
	syncInterval := 5
	if v := os.Getenv("SYNC_INTERVAL_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			syncInterval = n
		}
	}
	if d1.IsConfigured() {
		c := cron.New()
		spec := fmt.Sprintf("@every %dm", syncInterval)
		c.AddFunc(spec, func() {
			dirty, err := db.GetDirtyData()
			if err != nil {
				log.Printf("[SYNC] get dirty: %v", err)
				return
			}
			if len(dirty.Groups)+len(dirty.Sites)+len(dirty.Settings) == 0 {
				return
			}
			if err := d1.PushDirtyData(dirty); err != nil {
				db.LogSync("error", err.Error())
				log.Printf("[SYNC] push error: %v", err)
				return
			}
			db.ClearDirty()
			db.LogSync("ok", fmt.Sprintf("auto sync at %s", time.Now().Format(time.DateTime)))
			log.Printf("[SYNC] Auto sync OK — groups:%d sites:%d settings:%d",
				len(dirty.Groups), len(dirty.Sites), len(dirty.Settings))
		})
		c.Start()
		log.Printf("[SYNC] Auto sync every %d min", syncInterval)
	}

	// ── Gin 路由配置 ──────────────────────────────
	r := gin.Default()

	// CORS — 允许前端跨域访问
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

	// 静态前端文件
	r.Static("/app", frontendDir)
	r.StaticFile("/", frontendDir+"/index.html")

	api := r.Group("/api")
	{
		// 一次性返回全部初始化数据（减少前端请求次数）
		api.GET("/data", func(c *gin.Context) {
			engines, _ := db.GetSearchEngines()
			groups, _ := db.GetGroups()
			sites, _ := db.GetSites(0)
			settings, _ := db.GetSettings()
			c.JSON(200, gin.H{
				"search_engines": engines,
				"groups":         groups,
				"sites":          sites,
				"settings":       settings,
			})
		})

		// 搜索引擎
		api.GET("/search-engines", handlers.GetSearchEngines)
		api.PUT("/search-engines/:id/default", handlers.SetDefaultEngine)

		// 分组
		api.GET("/groups", handlers.GetGroups)
		api.POST("/groups", handlers.CreateGroup)
		api.PUT("/groups/:id", handlers.UpdateGroup)
		api.DELETE("/groups/:id", handlers.DeleteGroup)
		api.PUT("/groups/reorder", handlers.ReorderGroups)

		// 网站
		api.GET("/sites", handlers.GetSites)        // ?group_id=1 可过滤
		api.POST("/sites", handlers.CreateSite)
		api.PUT("/sites/:id", handlers.UpdateSite)
		api.DELETE("/sites/:id", handlers.DeleteSite)
		api.PUT("/sites/reorder", handlers.ReorderSites)

		// 配置
		api.GET("/settings", handlers.GetSettings)
		api.PUT("/settings/:key", handlers.SetSetting)

		// D1 同步
		api.POST("/sync/push", handlers.SyncToD1(d1))           // 手动推送到 D1
		api.POST("/sync/restore", handlers.RestoreFromD1(d1))   // 从 D1 恢复本地
		api.GET("/sync/status", handlers.GetD1Status(d1))       // D1 连接状态
		api.GET("/sync/logs", handlers.GetSyncLogs)             // 查看同步日志
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("[NAVI] Server running at http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
