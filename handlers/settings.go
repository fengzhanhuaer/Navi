// handlers/settings.go
// 配置 & D1 同步相关处理器

package handlers

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

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

	currExec, err := os.Executable()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取当前执行路径失败: " + err.Error()})
		return
	}

	// 在临时目录生成新版本执行文件，以备自测
	tmpNewExec := filepath.Join(tmpDir, "navi_new.exe")
	if runtime.GOOS != "windows" {
		tmpNewExec = filepath.Join(tmpDir, "navi_new")
	}

	// 提取二进制的辅助函数，将新程序写入到给定的路径并赋予执行权限
	extractBinary := func(src io.Reader, target string) error {
		newFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
		if err != nil {
			return fmt.Errorf("打开新文件失败: %v", err)
		}
		_, err = io.Copy(newFile, src)
		newFile.Close()
		if err != nil {
			return fmt.Errorf("写入新文件失败: %v", err)
		}
		return nil
	}

	var extracted bool
	if strings.HasSuffix(assetName, ".zip") {
		r, err := zip.OpenReader(tmpZipPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "zip解压失败: " + err.Error()})
			return
		}
		defer r.Close()

		for _, f := range r.File {
			if strings.HasSuffix(f.Name, "navi.exe") || strings.HasSuffix(f.Name, "navi") {
				rc, err := f.Open()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "读取zip内文件失败: " + err.Error()})
					return
				}
				if err := extractBinary(rc, tmpNewExec); err != nil {
					rc.Close()
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				rc.Close()
				extracted = true
				break
			}
		}
	} else if strings.HasSuffix(assetName, ".tar.gz") {
		f, err := os.Open(tmpZipPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "打开tar.gz失败: " + err.Error()})
			return
		}
		defer f.Close()
		gr, err := gzip.NewReader(f)
		if err == nil {
			defer gr.Close()
			tr := tar.NewReader(gr)
			for {
				hdr, err := tr.Next()
				if err == io.EOF { break }
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "读取tar文件失败: " + err.Error()})
					return
				}
				if strings.HasSuffix(hdr.Name, "navi.exe") || strings.HasSuffix(hdr.Name, "navi") {
					if err := extractBinary(tr, tmpNewExec); err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
						return
					}
					extracted = true
					break
				}
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "gzip解码失败: " + err.Error()})
			return
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "不支持的打包格式，预期为 .zip 或 .tar.gz"})
		return
	}

	if !extracted {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "压缩包中未找到 navi 执行文件"})
		return
	}

	// 执行安全自检测试
	checkCmd := exec.Command(tmpNewExec, "--check")
	if err := checkCmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "新版本的自检测试失败，升级回滚放弃执行: " + err.Error()})
		return
	}

	// 自检通过，开始实施安全替换操作
	oldExec := currExec + ".old"
	os.Remove(oldExec) // 忽略错误
	if err := os.Rename(currExec, oldExec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重命名旧程序失败，回滚: " + err.Error()})
		return
	}

	// 将自检通过的包覆写到真实执行路径
	if err := extractBinary(func() io.Reader {
		tr, _ := os.Open(tmpNewExec)
		return tr
	}(), currExec); err != nil {
		// 恢复原有的入口程序
		os.Rename(oldExec, currExec)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "替换新版本并赋予执行环境失败已被回滚: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "log": fmt.Sprintf("已成功从 GitHub 获取最新版本 %s 且安全验证完成。\n正在执行自动平滑重启...", *release.TagName)})

	go func() {
		// 延迟等待给客户端返回 HTTP 响应，然后自动重启。
		time.Sleep(2 * time.Second)
		if runtime.GOOS == "windows" {
			cmd := exec.Command(currExec, os.Args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Start()
			os.Exit(0)
		} else {
			// Linux 等系统下采用 syscall.Exec 进行无缝进程接管切换
			err := syscall.Exec(currExec, os.Args, os.Environ())
			if err != nil {
				// 替换接管失败尝试恢复旧二进制执行环境重启交由进程守护处理
				os.Rename(oldExec, currExec)
				os.Exit(1)
			}
		}
	}()
}

