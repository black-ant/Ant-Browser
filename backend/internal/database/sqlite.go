package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// DB 数据库连接
type DB struct {
	conn *sql.DB
}

// migration 单个版本迁移
type migration struct {
	version int    // 版本号，单调递增，永不修改
	desc    string // 描述，便于日志追踪
	stmts   []string
}

// migrations 所有版本迁移，按 version 升序排列
// 规则：
//   - 只能追加新版本，绝对不能修改已有版本
//   - version 从 1 开始，每次发布新版本时递增
//   - 每个 version 对应一批幂等的 DDL 语句
var migrations = []migration{
	{
		version: 1,
		desc:    "初始化核心表结构",
		stmts: []string{
			`CREATE TABLE IF NOT EXISTS launch_codes (
				profile_id TEXT PRIMARY KEY,
				code       TEXT NOT NULL UNIQUE,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_launch_codes_code ON launch_codes(code)`,

			`CREATE TABLE IF NOT EXISTS browser_profiles (
				profile_id       TEXT PRIMARY KEY,
				profile_name     TEXT NOT NULL,
				user_data_dir    TEXT NOT NULL DEFAULT '',
				core_id          TEXT NOT NULL DEFAULT '',
				fingerprint_args TEXT NOT NULL DEFAULT '[]',
				proxy_id         TEXT NOT NULL DEFAULT '',
				proxy_config     TEXT NOT NULL DEFAULT '',
				launch_args      TEXT NOT NULL DEFAULT '[]',
				tags             TEXT NOT NULL DEFAULT '[]',
				keywords         TEXT NOT NULL DEFAULT '[]',
				created_at       DATETIME NOT NULL,
				updated_at       DATETIME NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_browser_profiles_created_at ON browser_profiles(created_at)`,

			`CREATE TABLE IF NOT EXISTS browser_proxies (
				proxy_id     TEXT PRIMARY KEY,
				proxy_name   TEXT NOT NULL,
				proxy_config TEXT NOT NULL,
				dns_servers  TEXT NOT NULL DEFAULT '',
				sort_order   INTEGER NOT NULL DEFAULT 0,
				created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			`CREATE TABLE IF NOT EXISTS browser_cores (
				core_id    TEXT PRIMARY KEY,
				core_name  TEXT NOT NULL,
				core_path  TEXT NOT NULL,
				is_default INTEGER NOT NULL DEFAULT 0,
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,

			`CREATE TABLE IF NOT EXISTS browser_bookmarks (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				name       TEXT NOT NULL,
				url        TEXT NOT NULL UNIQUE,
				sort_order INTEGER NOT NULL DEFAULT 0
			)`,
		},
	},
	{
		version: 2,
		desc:    "添加实例分组支持",
		stmts: []string{
			`CREATE TABLE IF NOT EXISTS browser_groups (
				group_id   TEXT PRIMARY KEY,
				group_name TEXT NOT NULL,
				parent_id  TEXT DEFAULT '',
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_browser_groups_parent_id ON browser_groups(parent_id)`,
			`ALTER TABLE browser_profiles ADD COLUMN group_id TEXT DEFAULT ''`,
		},
	},
	{
		version: 3,
		desc:    "代理表添加分组和测速字段",
		stmts: []string{
			`ALTER TABLE browser_proxies ADD COLUMN group_name TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE browser_proxies ADD COLUMN last_latency_ms INTEGER NOT NULL DEFAULT -1`,
			`ALTER TABLE browser_proxies ADD COLUMN last_test_ok INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE browser_proxies ADD COLUMN last_tested_at TEXT NOT NULL DEFAULT ''`,
		},
	},
	{
		version: 4,
		desc:    "代理表添加 IP 健康结果字段",
		stmts: []string{
			`ALTER TABLE browser_proxies ADD COLUMN last_ip_health_json TEXT NOT NULL DEFAULT ''`,
		},
	},
	{
		version: 5,
		desc:    "代理表添加 URL 来源与自动刷新字段",
		stmts: []string{
			`ALTER TABLE browser_proxies ADD COLUMN source_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE browser_proxies ADD COLUMN source_url TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE browser_proxies ADD COLUMN source_name_prefix TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE browser_proxies ADD COLUMN source_auto_refresh INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE browser_proxies ADD COLUMN source_refresh_interval_m INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE browser_proxies ADD COLUMN source_last_refresh_at TEXT NOT NULL DEFAULT ''`,
		},
	},
	{
		version: 6,
		desc:    "实例表添加代理绑定快照字段",
		stmts: []string{
			`ALTER TABLE browser_profiles ADD COLUMN proxy_bind_source_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE browser_profiles ADD COLUMN proxy_bind_source_url TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE browser_profiles ADD COLUMN proxy_bind_name TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE browser_profiles ADD COLUMN proxy_bind_updated_at TEXT NOT NULL DEFAULT ''`,
		},
	},
	{
		version: 7,
		desc:    "书签表添加启动时打开字段",
		stmts: []string{
			`ALTER TABLE browser_bookmarks ADD COLUMN open_on_start INTEGER NOT NULL DEFAULT 0`,
		},
	},
	{
		version: 8,
		desc:    "添加 Chrome 插件包管理表",
		stmts: []string{
			`CREATE TABLE IF NOT EXISTS browser_extensions (
				extension_id  TEXT PRIMARY KEY,
				name          TEXT NOT NULL,
				version       TEXT NOT NULL DEFAULT '',
				description   TEXT NOT NULL DEFAULT '',
				manifest_json TEXT NOT NULL DEFAULT '{}',
				source_url    TEXT NOT NULL DEFAULT '',
				install_dir   TEXT NOT NULL,
				enabled       INTEGER NOT NULL DEFAULT 1,
				installed_at  TEXT NOT NULL DEFAULT '',
				updated_at    TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE INDEX IF NOT EXISTS idx_browser_extensions_enabled ON browser_extensions(enabled)`,
		},
	},
	{
		version: 9,
		desc:    "添加实例插件绑定表",
		stmts: []string{
			`CREATE TABLE IF NOT EXISTS browser_profile_extension_settings (
				profile_id  TEXT PRIMARY KEY,
				configured  INTEGER NOT NULL DEFAULT 0,
				updated_at  TEXT NOT NULL DEFAULT ''
			)`,
			`CREATE TABLE IF NOT EXISTS browser_profile_extensions (
				profile_id    TEXT NOT NULL,
				extension_id  TEXT NOT NULL,
				enabled       INTEGER NOT NULL DEFAULT 1,
				created_at    TEXT NOT NULL DEFAULT '',
				updated_at    TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (profile_id, extension_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_browser_profile_extensions_profile ON browser_profile_extensions(profile_id)`,
			`CREATE INDEX IF NOT EXISTS idx_browser_profile_extensions_extension ON browser_profile_extensions(extension_id)`,
		},
	},
	{
		version: 10,
		desc:    "插件表添加图标缓存字段",
		stmts: []string{
			`ALTER TABLE browser_extensions ADD COLUMN icon_data_url TEXT NOT NULL DEFAULT ''`,
		},
	},
	{
		version: 11,
		desc:    "实例表添加回收站字段",
		stmts: []string{
			`ALTER TABLE browser_profiles ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`,
			`CREATE INDEX IF NOT EXISTS idx_browser_profiles_deleted_at ON browser_profiles(deleted_at)`,
		},
	},
	{
		version: 12,
		desc:    "代理表添加指定内核字段",
		stmts: []string{
			`ALTER TABLE browser_proxies ADD COLUMN preferred_kernel TEXT NOT NULL DEFAULT ''`,
		},
	},
	{
		version: 13,
		desc:    "实例表添加历史标签恢复覆盖字段",
		stmts: []string{
			`ALTER TABLE browser_profiles ADD COLUMN restore_last_session TEXT NOT NULL DEFAULT ''`,
		},
	},
	{
		version: 14,
		desc:    "实例表添加内存限制字段",
		stmts: []string{
			`ALTER TABLE browser_profiles ADD COLUMN memory_limit_mb INTEGER NOT NULL DEFAULT 0`,
		},
	},
	{
		version: 15,
		desc:    "添加实例代理分流规则",
		stmts: []string{
			`CREATE TABLE IF NOT EXISTS browser_profile_proxy_routing (
				profile_id TEXT PRIMARY KEY,
				mode TEXT NOT NULL DEFAULT 'proxy',
				rules_json TEXT NOT NULL DEFAULT '[]',
				updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (profile_id) REFERENCES browser_profiles(profile_id) ON DELETE CASCADE
			)`,
		},
	},
	// ── 新版本在此追加，格式：
	// {
	//     version: 4,
	//     desc:    "描述本次变更",
	//     stmts: []string{
	//         `ALTER TABLE xxx ADD COLUMN yyy TEXT NOT NULL DEFAULT ''`,
	//     },
	// },
}

// NewDB 创建新的数据库连接
func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// WAL 模式：写不阻塞读
	if _, err := conn.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return nil, fmt.Errorf("设置 WAL 模式失败: %w", err)
	}
	// 开启外键约束
	if _, err := conn.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return nil, fmt.Errorf("开启外键约束失败: %w", err)
	}

	return &DB{conn: conn}, nil
}

// GetConn 获取数据库连接
func (db *DB) GetConn() *sql.DB {
	return db.conn
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Migrate 执行版本化迁移
// 原理：维护 schema_migrations 表记录已执行版本，每次启动只执行未执行的版本
func (db *DB) Migrate() error {
	// 确保版本记录表存在
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			desc       TEXT NOT NULL DEFAULT '',
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return fmt.Errorf("创建 schema_migrations 表失败: %w", err)
	}

	// 查询已执行的最大版本号
	var currentVersion int
	row := db.conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("查询当前 schema 版本失败: %w", err)
	}

	// 按版本顺序执行未执行的迁移
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue // 已执行，跳过
		}

		// 每个版本在事务内执行，保证原子性
		if err := db.applyMigration(m); err != nil {
			return fmt.Errorf("迁移版本 %d (%s) 失败: %w", m.version, m.desc, err)
		}
	}

	return nil
}

// applyMigration 在事务内执行单个版本的所有语句，并记录版本号
func (db *DB) applyMigration(m migration) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range m.stmts {
		if _, err := tx.Exec(stmt); err != nil {
			// ALTER TABLE 添加已存在列时忽略（兼容从旧版本直接升级的情况）
			if isColumnExistsError(err) {
				continue
			}
			return fmt.Errorf("执行语句失败 [%s]: %w", truncate(stmt, 60), err)
		}
	}

	// 记录版本号
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, desc) VALUES (?, ?)`,
		m.version, m.desc,
	); err != nil {
		return fmt.Errorf("记录迁移版本失败: %w", err)
	}

	return tx.Commit()
}

// isColumnExistsError 检查是否是列已存在的错误（SQLite 错误信息）
func isColumnExistsError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "duplicate column") || strings.Contains(s, "already exists")
}

// truncate 截断字符串用于日志展示
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
