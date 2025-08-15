package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---- モデル・定数 ----

type Event struct {
	ID           int       `json:"id"`
	Title        string    `json:"title"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	Lat          float64   `json:"lat"`
	Lng          float64   `json:"lng"`
	Location     string    `json:"location"`
	SiteUrl      string    `json:"site_url"`
	Category     string    `json:"category"`
	IconCategory int       `json:"icon_category"`
}

type Review struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	EventID   int       `json:"event_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}


const dataDir = "data"

var fileMu sync.Mutex // JSON 生成の競合を防止

// ---- ユーティリティ ----

func mustJST() *time.Location {
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return loc
	}
	return time.FixedZone("JST", 9*60*60)
}

func yyyymmdd(t time.Time) string { return t.Format("20060102") }

func jsonPathForDate(t time.Time) string {
	return filepath.Join(dataDir, fmt.Sprintf("events-%s.json", yyyymmdd(t)))
}

func nextJSTMidnightPlus(t time.Time, plusMin int) time.Time {
	jst := mustJST()
	n := t.In(jst)
	// 翌日の 00:plusMin:00 JST
	return time.Date(n.Year(), n.Month(), n.Day()+1, 0, plusMin, 0, 0, jst)
}

// ---- JSON 読み書き＆日次処理 ----

func readEventsFromFile(path string) ([]Event, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(b, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func ensureTodayFileAndCleanup() error {
	jst := mustJST()
	today := time.Now().In(jst)
	if _, err := os.Stat(jsonPathForDate(today)); os.IsNotExist(err) {
		if err := writeJsonForDateFromDB(today); err != nil {
			return err
		}
	}
	return cleanupOldFiles(today)
}

func cleanupOldFiles(today time.Time) error {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return err
	}
	todayName := filepath.Base(jsonPathForDate(today))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == todayName {
			continue
		}
		if !strings.HasPrefix(name, "events-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		_ = os.Remove(filepath.Join(dataDir, name))
	}
	return nil
}

func writeJsonForDateFromDB(day time.Time) error {
	fileMu.Lock()
	defer fileMu.Unlock()

	path := jsonPathForDate(day)
	// 既に誰かが作っていればスキップ
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	events, err := fetchEventsForDate(day)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// ---- DB アクセス（イベント取得）----

func fetchEventsForDate(day time.Time) ([]Event, error) {
	dateStr := day.Format("2006-01-02")

	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, fmt.Errorf("DB接続失敗: %w", err)
	}
	defer conn.Close(context.Background())

	q := `
		SELECT id, title, start_date, end_date, ST_AsText(lnglat), location, category, site_url, icon_category
		FROM events
		WHERE start_date <= $1::date AND end_date >= $1::date
		ORDER BY start_date, title
	`
	rows, err := conn.Query(context.Background(), q, dateStr)
	if err != nil {
		return nil, fmt.Errorf("クエリ失敗: %w", err)
	}
	defer rows.Close()

	var results []Event
	for rows.Next() {
		var e Event
		var point string
		if err := rows.Scan(&e.ID, &e.Title, &e.StartDate, &e.EndDate, &point, &e.Location, &e.Category, &e.SiteUrl, &e.IconCategory); err != nil {
			return nil, fmt.Errorf("スキャン失敗: %w", err)
		}
		// POINT(lng lat) → e.Lng / e.Lat
		point = strings.TrimPrefix(point, "POINT(")
		point = strings.TrimSuffix(point, ")")
		parts := strings.Split(point, " ")
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%f", &e.Lng)
			fmt.Sscanf(parts[1], "%f", &e.Lat)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// ---- DB アクセス（メッセージ挿入）----

func insertEventMessage(ctx context.Context, userID string, eventID int, comment string, rating int) error {
	fmt.Println("erroraaaaaafefweasfsedgagsa")
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	fmt.Println("erroraaaaaafefweasfsedgagsa")
	if err != nil {

		return fmt.Errorf("DB接続失敗: %w", err)
	}
	defer conn.Close(ctx)

	const q = `
		INSERT INTO reviews (event_id, user_id, comment, rating)
		VALUES ($1, $2, $3, $4)
	`
	fmt.Println("error:")

	// Exec で実行
	_, err = conn.Exec(ctx, q, eventID, userID, comment, rating)
	fmt.Println("error:", err)
	if err != nil {
		// 詳細ログを出力
		fmt.Printf("insert error (raw): %+v\n", err)
		if pgErr, ok := err.(*pgconn.PgError); ok {
			fmt.Printf("insert error detail: Code=%s Message=%s Detail=%s Where=%s\n",
				pgErr.Code, pgErr.Message, pgErr.Detail, pgErr.Where)
			return fmt.Errorf("insert失敗: code=%s message=%s detail=%s where=%s",
				pgErr.Code, pgErr.Message, pgErr.Detail, pgErr.Where)
		}
		return fmt.Errorf("insert失敗: %w", err)
	}

	fmt.Println("insert成功:", eventID, userID, comment, rating)
	return nil
}

func queryEventReviews(ctx context.Context, eventID int) ([]Review, error) {
	// 都度コネクションを開く（必要に応じてコネクションプール化推奨）
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, fmt.Errorf("DB接続失敗: %w", err)
	}
	defer conn.Close(ctx)

	const q = `
		SELECT id, user_id, event_id, rating, comment, created_at
		FROM reviews
		WHERE event_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := conn.Query(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("select失敗: %w", err)
	}
	defer rows.Close()

	var reviews []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.UserID, &r.EventID, &r.Rating, &r.Comment, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan失敗: %w", err)
		}
		reviews = append(reviews, r)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("rowsエラー: %w", rows.Err())
	}

	return reviews, nil
}