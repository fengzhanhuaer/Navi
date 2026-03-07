// db/local.go
// 本地 SQLite 数据库层：所有增删改查操作
// 使用 modernc.org/sqlite（纯 Go，无需 CGo）

package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB 是全局数据库连接（包内共享）
var DB *sql.DB

// ─────────────────────────────────────────────
// 数据模型
// ─────────────────────────────────────────────

type SiteGroup struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Icon       string    `json:"icon"`
	OrderIndex int       `json:"order_index"`
	Collapsed  bool      `json:"collapsed"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Site struct {
	ID         int64     `json:"id"`
	GroupID    int64     `json:"group_id"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	Icon       string    `json:"icon"`
	OrderIndex int       `json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

// DirtyData 用于 D1 同步
type DirtyData struct {
	Groups   []SiteGroup       `json:"groups"`
	Sites    []Site            `json:"sites"`
	Settings []Setting         `json:"settings"`
}

// ─────────────────────────────────────────────
// 初始化
// ─────────────────────────────────────────────

func Init(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "navi.db")
	var err error
	DB, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	if err = createTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	if err = seedDefaults(); err != nil {
		return fmt.Errorf("seed defaults: %w", err)
	}

	log.Printf("[DB] SQLite ready at %s", dbPath)
	return nil
}

func createTables() error {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS site_groups (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT    NOT NULL,
			icon        TEXT    DEFAULT '📁',
			order_index INTEGER DEFAULT 0,
			collapsed   INTEGER DEFAULT 0,
			created_at  TEXT    DEFAULT (datetime('now')),
			updated_at  TEXT    DEFAULT (datetime('now')),
			dirty       INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS sites (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id    INTEGER REFERENCES site_groups(id) ON DELETE CASCADE,
			title       TEXT    NOT NULL,
			url         TEXT    NOT NULL,
			icon        TEXT    DEFAULT '',
			order_index INTEGER DEFAULT 0,
			created_at  TEXT    DEFAULT (datetime('now')),
			updated_at  TEXT    DEFAULT (datetime('now')),
			dirty       INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at TEXT DEFAULT (datetime('now')),
			dirty      INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS sync_log (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			synced_at TEXT DEFAULT (datetime('now')),
			status    TEXT,
			message   TEXT
		);

		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at    TEXT DEFAULT (datetime('now'))
		);
	`)
	return err
}

// ─────────────────────────────────────────────
// 用户认证
// ─────────────────────────────────────────────

// UserExists 返回 true 表示已有注册用户（只允许一个）
func UserExists() bool {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0
}

// CreateUser 插入新用户（已有用户时返回错误）
func CreateUser(username, passwordHash string) error {
	if UserExists() {
		return fmt.Errorf("already registered")
	}
	_, err := DB.Exec("INSERT INTO users (username, password_hash) VALUES (?,?)", username, passwordHash)
	return err
}

// GetUserByUsername 按用户名查询用户
func GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := DB.QueryRow("SELECT id, username, password_hash FROM users WHERE username=?", username).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func seedDefaults() error {
	var count int

	_ = DB.QueryRow("SELECT COUNT(*) FROM site_groups").Scan(&count)
	if count == 0 {
		res, _ := DB.Exec("INSERT INTO site_groups (name,icon,order_index) VALUES ('常用网站','⭐',0)")
		gid, _ := res.LastInsertId()
		defaults := []struct{ title, url string }{
			{"GitHub",  "https://github.com"},
			{"YouTube", "https://youtube.com"},
			{"Google",  "https://google.com"},
		}
		for i, s := range defaults {
			DB.Exec("INSERT INTO sites (group_id,title,url,order_index) VALUES (?,?,?,?)", gid, s.title, s.url, i)
		}
	}

	DB.Exec("INSERT OR IGNORE INTO settings (key,value) VALUES ('theme','dark')")
	DB.Exec("INSERT OR IGNORE INTO settings (key,value) VALUES ('background','gradient')")
	return nil
}

// ─────────────────────────────────────────────
// 分组
// ─────────────────────────────────────────────

func GetGroups() ([]SiteGroup, error) {
	rows, err := DB.Query("SELECT id,name,icon,order_index,collapsed,created_at,updated_at FROM site_groups ORDER BY order_index")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []SiteGroup
	for rows.Next() {
		var g SiteGroup
		var collapsed int
		var createdAt, updatedAt string
		rows.Scan(&g.ID, &g.Name, &g.Icon, &g.OrderIndex, &collapsed, &createdAt, &updatedAt)
		g.Collapsed = collapsed == 1
		g.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		g.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		list = append(list, g)
	}
	return list, nil
}

func CreateGroup(name, icon string) (int64, error) {
	var maxOrder int
	DB.QueryRow("SELECT COALESCE(MAX(order_index),0) FROM site_groups").Scan(&maxOrder)
	res, err := DB.Exec(
		"INSERT INTO site_groups (name,icon,order_index,dirty) VALUES (?,?,?,1)",
		name, icon, maxOrder+1,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateGroup(id int64, name, icon string, collapsed bool) error {
	c := 0
	if collapsed { c = 1 }
	_, err := DB.Exec(
		"UPDATE site_groups SET name=?,icon=?,collapsed=?,updated_at=datetime('now'),dirty=1 WHERE id=?",
		name, icon, c, id,
	)
	return err
}

func DeleteGroup(id int64) error {
	_, err := DB.Exec("DELETE FROM site_groups WHERE id=?", id)
	return err
}

func ReorderGroups(items []struct{ ID int64; Order int }) error {
	tx, _ := DB.Begin()
	for _, item := range items {
		tx.Exec("UPDATE site_groups SET order_index=?,dirty=1 WHERE id=?", item.Order, item.ID)
	}
	return tx.Commit()
}

// ─────────────────────────────────────────────
// 网站
// ─────────────────────────────────────────────

func GetSites(groupID int64) ([]Site, error) {
	var rows *sql.Rows
	var err error
	if groupID > 0 {
		rows, err = DB.Query(
			"SELECT id,group_id,title,url,icon,order_index,created_at,updated_at FROM sites WHERE group_id=? ORDER BY order_index",
			groupID,
		)
	} else {
		rows, err = DB.Query(
			"SELECT id,group_id,title,url,icon,order_index,created_at,updated_at FROM sites ORDER BY group_id,order_index",
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Site
	for rows.Next() {
		var s Site
		var createdAt, updatedAt string
		rows.Scan(&s.ID, &s.GroupID, &s.Title, &s.URL, &s.Icon, &s.OrderIndex, &createdAt, &updatedAt)
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		list = append(list, s)
	}
	return list, nil
}

func CreateSite(groupID int64, title, url, icon string) (int64, error) {
	var maxOrder int
	DB.QueryRow("SELECT COALESCE(MAX(order_index),0) FROM sites WHERE group_id=?", groupID).Scan(&maxOrder)
	res, err := DB.Exec(
		"INSERT INTO sites (group_id,title,url,icon,order_index,dirty) VALUES (?,?,?,?,?,1)",
		groupID, title, url, icon, maxOrder+1,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateSite(id, groupID int64, title, url, icon string) error {
	_, err := DB.Exec(
		"UPDATE sites SET group_id=?,title=?,url=?,icon=?,updated_at=datetime('now'),dirty=1 WHERE id=?",
		groupID, title, url, icon, id,
	)
	return err
}

func DeleteSite(id int64) error {
	_, err := DB.Exec("DELETE FROM sites WHERE id=?", id)
	return err
}

func ReorderSites(items []struct{ ID int64; Order int }) error {
	tx, _ := DB.Begin()
	for _, item := range items {
		tx.Exec("UPDATE sites SET order_index=?,dirty=1 WHERE id=?", item.Order, item.ID)
	}
	return tx.Commit()
}

// ─────────────────────────────────────────────
// 配置
// ─────────────────────────────────────────────

func GetSettings() (map[string]string, error) {
	rows, err := DB.Query("SELECT key,value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		m[k] = v
	}
	return m, nil
}

func SetSetting(key, value string) error {
	_, err := DB.Exec(
		"INSERT OR REPLACE INTO settings (key,value,updated_at,dirty) VALUES (?,?,datetime('now'),1)",
		key, value,
	)
	return err
}

// GetSettingPublic 读取单个配置项（用于后端内部展示，不标记 dirty）
func GetSettingPublic(key string) (string, error) {
	var v string
	err := DB.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&v)
	return v, err
}


// ─────────────────────────────────────────────
// D1 同步辅助
// ─────────────────────────────────────────────

func GetDirtyData() (*DirtyData, error) {
	d := &DirtyData{}

	// dirty groups
	rows, err := DB.Query("SELECT id,name,icon,order_index,collapsed,created_at,updated_at FROM site_groups WHERE dirty=1")
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var g SiteGroup
		var c int; var ca, ua string
		rows.Scan(&g.ID, &g.Name, &g.Icon, &g.OrderIndex, &c, &ca, &ua)
		g.Collapsed = c == 1
		g.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ca)
		g.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", ua)
		d.Groups = append(d.Groups, g)
	}

	// dirty sites
	rows2, err := DB.Query("SELECT id,group_id,title,url,icon,order_index,created_at,updated_at FROM sites WHERE dirty=1")
	if err != nil { return nil, err }
	defer rows2.Close()
	for rows2.Next() {
		var s Site; var ca, ua string
		rows2.Scan(&s.ID, &s.GroupID, &s.Title, &s.URL, &s.Icon, &s.OrderIndex, &ca, &ua)
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ca)
		s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", ua)
		d.Sites = append(d.Sites, s)
	}

	// dirty settings
	rows3, err := DB.Query("SELECT key,value FROM settings WHERE dirty=1")
	if err != nil { return nil, err }
	defer rows3.Close()
	for rows3.Next() {
		var s Setting
		rows3.Scan(&s.Key, &s.Value)
		d.Settings = append(d.Settings, s)
	}

	return d, nil
}

func ClearDirty() error {
	tx, _ := DB.Begin()
	tx.Exec("UPDATE site_groups SET dirty=0")
	tx.Exec("UPDATE sites SET dirty=0")
	tx.Exec("UPDATE settings SET dirty=0")
	return tx.Commit()
}

func LogSync(status, message string) {
	DB.Exec("INSERT INTO sync_log (status,message) VALUES (?,?)", status, message)
}

func GetSyncLogs(limit int) ([]map[string]string, error) {
	rows, err := DB.Query("SELECT synced_at,status,message FROM sync_log ORDER BY id DESC LIMIT ?", limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var logs []map[string]string
	for rows.Next() {
		var synced, status, message string
		rows.Scan(&synced, &status, &message)
		logs = append(logs, map[string]string{"synced_at": synced, "status": status, "message": message})
	}
	return logs, nil
}
