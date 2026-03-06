// db/d1.go
// Cloudflare D1 同步模块
// 通过 D1 HTTP API 将本地 dirty 数据备份到云端

package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// D1Config 存储 Cloudflare 配置
type D1Config struct {
	AccountID  string
	DatabaseID string
	APIToken   string
}

// D1HTTPClient D1 操作客户端
type D1HTTPClient struct {
	cfg    D1Config
	client *http.Client
}

// NewD1Client 从环境变量创建 D1 客户端
func NewD1Client() *D1HTTPClient {
	return &D1HTTPClient{
		cfg: D1Config{
			AccountID:  os.Getenv("CF_ACCOUNT_ID"),
			DatabaseID: os.Getenv("CF_D1_DATABASE_ID"),
			APIToken:   os.Getenv("CF_API_TOKEN"),
		},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *D1HTTPClient) IsConfigured() bool {
	return c.cfg.AccountID != "" &&
		c.cfg.DatabaseID != "" &&
		c.cfg.APIToken != ""
}

// d1Request 向 D1 API 发送 SQL 查询
func (c *D1HTTPClient) d1Request(statements []d1Statement) (*d1Response, error) {
	url := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
		c.cfg.AccountID, c.cfg.DatabaseID,
	)

	body, _ := json.Marshal(map[string]any{"sql": statements[0].SQL, "params": statements[0].Params})
	if len(statements) > 1 {
		body, _ = json.Marshal(statements)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result d1Response
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("D1 API error: %v", result.Errors)
	}
	return &result, nil
}

type d1Statement struct {
	SQL    string `json:"sql"`
	Params []any  `json:"params,omitempty"`
}

type d1Response struct {
	Success bool        `json:"success"`
	Errors  []d1Error   `json:"errors"`
	Result  []d1Result  `json:"result"`
}

type d1Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type d1Result struct {
	Results []map[string]any `json:"results"`
	Success bool             `json:"success"`
}

// ─────────────────────────────────────────────
// D1 表初始化
// ─────────────────────────────────────────────

// InitD1Tables 在 D1 中创建表（首次使用时调用）
func (c *D1HTTPClient) InitD1Tables() error {
	if !c.IsConfigured() {
		return fmt.Errorf("D1 not configured")
	}

	sqls := []string{
		`CREATE TABLE IF NOT EXISTS site_groups (
			id INTEGER PRIMARY KEY,name TEXT NOT NULL,icon TEXT DEFAULT '📁',
			order_index INTEGER DEFAULT 0,collapsed INTEGER DEFAULT 0,
			created_at TEXT,updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sites (
			id INTEGER PRIMARY KEY,group_id INTEGER,title TEXT NOT NULL,
			url TEXT NOT NULL,icon TEXT DEFAULT '',order_index INTEGER DEFAULT 0,
			created_at TEXT,updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY,value TEXT NOT NULL,updated_at TEXT)`,
	}

	for _, sql := range sqls {
		if _, err := c.d1Request([]d1Statement{{SQL: sql}}); err != nil {
			return fmt.Errorf("init table: %w", err)
		}
	}
	log.Println("[D1] Tables initialized")
	return nil
}

// ─────────────────────────────────────────────
// 增量同步：将 dirty 数据 push 到 D1
// ─────────────────────────────────────────────

func (c *D1HTTPClient) PushDirtyData(data *DirtyData) error {
	if !c.IsConfigured() {
		return fmt.Errorf("D1 not configured, skipping sync")
	}

	var stmts []d1Statement

	// upsert groups
	for _, g := range data.Groups {
		c := 0
		if g.Collapsed { c = 1 }
		stmts = append(stmts, d1Statement{
			SQL: `INSERT OR REPLACE INTO site_groups
				(id,name,icon,order_index,collapsed,created_at,updated_at)
				VALUES (?,?,?,?,?,?,?)`,
			Params: []any{g.ID, g.Name, g.Icon, g.OrderIndex, c,
				g.CreatedAt.Format(time.DateTime), g.UpdatedAt.Format(time.DateTime)},
		})
	}

	// upsert sites
	for _, s := range data.Sites {
		stmts = append(stmts, d1Statement{
			SQL: `INSERT OR REPLACE INTO sites
				(id,group_id,title,url,icon,order_index,created_at,updated_at)
				VALUES (?,?,?,?,?,?,?,?)`,
			Params: []any{s.ID, s.GroupID, s.Title, s.URL, s.Icon, s.OrderIndex,
				s.CreatedAt.Format(time.DateTime), s.UpdatedAt.Format(time.DateTime)},
		})
	}

	// upsert settings
	for _, s := range data.Settings {
		stmts = append(stmts, d1Statement{
			SQL:    `INSERT OR REPLACE INTO settings (key,value,updated_at) VALUES (?,?,datetime('now'))`,
			Params: []any{s.Key, s.Value},
		})
	}

	if len(stmts) == 0 {
		return nil // 无需同步
	}

	// D1 HTTP API 每次最多 100 条语句，分批发送
	batchSize := 100
	for i := 0; i < len(stmts); i += batchSize {
		end := i + batchSize
		if end > len(stmts) { end = len(stmts) }
		batch := stmts[i:end]

		// 逐条发送（D1 免费版不支持批量事务）
		for _, stmt := range batch {
			if _, err := c.d1Request([]d1Statement{stmt}); err != nil {
				return fmt.Errorf("push to D1: %w", err)
			}
		}
	}

	log.Printf("[D1] Pushed %d groups, %d sites, %d settings",
		len(data.Groups), len(data.Sites), len(data.Settings))
	return nil
}

// ─────────────────────────────────────────────
// 从 D1 拉取完整数据（新设备恢复用）
// ─────────────────────────────────────────────

func (c *D1HTTPClient) PullAll() (*DirtyData, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("D1 not configured")
	}

	data := &DirtyData{}

	// 拉取分组
	resp, err := c.d1Request([]d1Statement{
		{SQL: "SELECT id,name,icon,order_index,collapsed,created_at,updated_at FROM site_groups ORDER BY order_index"},
	})
	if err != nil { return nil, err }
	if len(resp.Result) > 0 {
		for _, row := range resp.Result[0].Results {
			g := SiteGroup{}
			if v, ok := row["id"].(float64); ok { g.ID = int64(v) }
			if v, ok := row["name"].(string); ok { g.Name = v }
			if v, ok := row["icon"].(string); ok { g.Icon = v }
			if v, ok := row["order_index"].(float64); ok { g.OrderIndex = int(v) }
			if v, ok := row["collapsed"].(float64); ok { g.Collapsed = v == 1 }
			data.Groups = append(data.Groups, g)
		}
	}

	// 拉取网站
	resp2, err := c.d1Request([]d1Statement{
		{SQL: "SELECT id,group_id,title,url,icon,order_index FROM sites ORDER BY group_id,order_index"},
	})
	if err != nil { return nil, err }
	if len(resp2.Result) > 0 {
		for _, row := range resp2.Result[0].Results {
			s := Site{}
			if v, ok := row["id"].(float64); ok { s.ID = int64(v) }
			if v, ok := row["group_id"].(float64); ok { s.GroupID = int64(v) }
			if v, ok := row["title"].(string); ok { s.Title = v }
			if v, ok := row["url"].(string); ok { s.URL = v }
			if v, ok := row["icon"].(string); ok { s.Icon = v }
			if v, ok := row["order_index"].(float64); ok { s.OrderIndex = int(v) }
			data.Sites = append(data.Sites, s)
		}
	}

	// 拉取配置
	resp3, err := c.d1Request([]d1Statement{
		{SQL: "SELECT key,value FROM settings"},
	})
	if err != nil { return nil, err }
	if len(resp3.Result) > 0 {
		for _, row := range resp3.Result[0].Results {
			s := Setting{}
			if v, ok := row["key"].(string); ok { s.Key = v }
			if v, ok := row["value"].(string); ok { s.Value = v }
			data.Settings = append(data.Settings, s)
		}
	}

	log.Printf("[D1] Pulled %d groups, %d sites, %d settings",
		len(data.Groups), len(data.Sites), len(data.Settings))
	return data, nil
}

// ─────────────────────────────────────────────
// 从 D1 数据恢复本地 SQLite
// ─────────────────────────────────────────────

func RestoreFromD1Data(data *DirtyData) error {
	tx, err := DB.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	tx.Exec("DELETE FROM sites")
	tx.Exec("DELETE FROM site_groups")
	tx.Exec("DELETE FROM settings")

	for _, g := range data.Groups {
		c := 0
		if g.Collapsed { c = 1 }
		tx.Exec(
			"INSERT OR REPLACE INTO site_groups (id,name,icon,order_index,collapsed) VALUES (?,?,?,?,?)",
			g.ID, g.Name, g.Icon, g.OrderIndex, c,
		)
	}

	for _, s := range data.Sites {
		tx.Exec(
			"INSERT OR REPLACE INTO sites (id,group_id,title,url,icon,order_index) VALUES (?,?,?,?,?,?)",
			s.ID, s.GroupID, s.Title, s.URL, s.Icon, s.OrderIndex,
		)
	}

	for _, s := range data.Settings {
		tx.Exec("INSERT OR REPLACE INTO settings (key,value) VALUES (?,?)", s.Key, s.Value)
	}

	return tx.Commit()
}

// GetD1Status 测试 D1 连接是否正常
func (c *D1HTTPClient) GetD1Status() (string, error) {
	if !c.IsConfigured() {
		return "not_configured", nil
	}
	resp, err := c.d1Request([]d1Statement{{SQL: "SELECT 1 as ok"}})
	if err != nil {
		return "error", err
	}
	if resp.Success {
		return "ok", nil
	}
	msgs := make([]string, len(resp.Errors))
	for i, e := range resp.Errors { msgs[i] = e.Message }
	return "error", fmt.Errorf(strings.Join(msgs, "; "))
}
