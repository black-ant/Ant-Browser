package browser

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"ant-chrome/backend/internal/config"
)

// BookmarkDAO 默认书签持久化接口
type BookmarkDAO interface {
	List() ([]config.BrowserBookmark, error)
	ReplaceAll(bookmarks []config.BrowserBookmark) error
}

type BookmarkDeletionDAO interface {
	DeletedURLs() ([]string, error)
}

type BookmarkDeletionWriter interface {
	SyncDeletedURLs(urls []string) error
}

// SQLiteBookmarkDAO 基于 SQLite 的 BookmarkDAO 实现
type SQLiteBookmarkDAO struct {
	db *sql.DB
}

// NewSQLiteBookmarkDAO 创建 SQLiteBookmarkDAO
func NewSQLiteBookmarkDAO(db *sql.DB) *SQLiteBookmarkDAO {
	return &SQLiteBookmarkDAO{db: db}
}

// List 查询所有默认书签，按 sort_order 升序
func (d *SQLiteBookmarkDAO) List() ([]config.BrowserBookmark, error) {
	rows, err := d.db.Query(`
		SELECT name, url, COALESCE(open_on_start, 0) FROM browser_bookmarks ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询书签列表失败: %w", err)
	}
	defer rows.Close()

	var list []config.BrowserBookmark
	for rows.Next() {
		var b config.BrowserBookmark
		var openOnStart int
		if err := rows.Scan(&b.Name, &b.URL, &openOnStart); err != nil {
			return nil, fmt.Errorf("读取书签行失败: %w", err)
		}
		b.OpenOnStart = openOnStart != 0
		list = append(list, b)
	}
	return list, rows.Err()
}

// ReplaceAll 原子替换全部书签（事务保证）
func (d *SQLiteBookmarkDAO) ReplaceAll(bookmarks []config.BrowserBookmark) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	existingURLs, err := readBookmarkURLKeys(tx)
	if err != nil {
		return fmt.Errorf("读取现有书签失败: %w", err)
	}

	keptURLs := make(map[string]struct{}, len(bookmarks))
	for _, bookmark := range bookmarks {
		key := NormalizeBookmarkURL(bookmark.URL)
		if key != "" {
			keptURLs[key] = struct{}{}
		}
	}
	for url := range existingURLs {
		if _, ok := keptURLs[url]; ok || url == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO browser_bookmark_deletions (url) VALUES (?)`, url); err != nil {
			return fmt.Errorf("记录已删除书签失败: %w", err)
		}
	}

	if _, err := tx.Exec(`DELETE FROM browser_bookmarks`); err != nil {
		return fmt.Errorf("清空书签失败: %w", err)
	}
	for i, b := range bookmarks {
		if strings.TrimSpace(b.Name) == "" || strings.TrimSpace(b.URL) == "" {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM browser_bookmark_deletions WHERE url = ?`, NormalizeBookmarkURL(b.URL)); err != nil {
			return fmt.Errorf("清除已恢复书签标记失败: %w", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order) VALUES (?, ?, ?, ?)`,
			b.Name, b.URL, boolToInt(b.OpenOnStart), i,
		); err != nil {
			return fmt.Errorf("插入书签失败: %w", err)
		}
	}
	return tx.Commit()
}

func (d *SQLiteBookmarkDAO) DeletedURLs() ([]string, error) {
	rows, err := d.db.Query(`SELECT url FROM browser_bookmark_deletions ORDER BY url`)
	if err != nil {
		return nil, fmt.Errorf("查询已删除书签失败: %w", err)
	}
	defer rows.Close()

	urls := make([]string, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var rawURL string
		if err := rows.Scan(&rawURL); err != nil {
			return nil, fmt.Errorf("读取已删除书签失败: %w", err)
		}
		bookmarkURL := NormalizeBookmarkURL(rawURL)
		if bookmarkURL == "" {
			continue
		}
		if _, ok := seen[bookmarkURL]; ok {
			continue
		}
		seen[bookmarkURL] = struct{}{}
		urls = append(urls, bookmarkURL)
	}
	return urls, rows.Err()
}

func (d *SQLiteBookmarkDAO) SyncDeletedURLs(urls []string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开启删除标记事务失败: %w", err)
	}
	defer tx.Rollback()

	activeURLs, err := readBookmarkURLKeys(tx)
	if err != nil {
		return fmt.Errorf("读取活动书签失败: %w", err)
	}
	desiredURLs := make(map[string]struct{}, len(urls))
	for _, rawURL := range urls {
		bookmarkURL := NormalizeBookmarkURL(rawURL)
		if bookmarkURL == "" {
			continue
		}
		if _, active := activeURLs[bookmarkURL]; active {
			continue
		}
		desiredURLs[bookmarkURL] = struct{}{}
	}
	if _, err := tx.Exec(`DELETE FROM browser_bookmark_deletions`); err != nil {
		return fmt.Errorf("清理旧书签删除标记失败: %w", err)
	}
	for bookmarkURL := range desiredURLs {
		if _, err := tx.Exec(`INSERT INTO browser_bookmark_deletions (url) VALUES (?)`, bookmarkURL); err != nil {
			return fmt.Errorf("写入已删除书签失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交删除标记事务失败: %w", err)
	}
	return nil
}

func NormalizeBookmarkURL(rawURL string) string {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return ""
	}
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil || parsedURL.Scheme == "" {
		return trimmedURL
	}
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	if parsedURL.Host != "" {
		parsedURL.Host = strings.ToLower(parsedURL.Host)
	}
	return parsedURL.String()
}

type bookmarkURLQuerier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func readBookmarkURLKeys(queryer bookmarkURLQuerier) (map[string]struct{}, error) {
	rows, err := queryer.Query(`SELECT url FROM browser_bookmarks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := make(map[string]struct{})
	for rows.Next() {
		var rawURL string
		if err := rows.Scan(&rawURL); err != nil {
			return nil, err
		}
		bookmarkURL := NormalizeBookmarkURL(rawURL)
		if bookmarkURL != "" {
			urls[bookmarkURL] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
