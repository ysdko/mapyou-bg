package main

import (
	"context"
	"fmt"
	"os"
	"strings"
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



// ---- ユーティリティ ----

func mustJST() *time.Location {
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return loc
	}
	return time.FixedZone("JST", 9*60*60)
}


// ---- JSON 読み書き＆日次処理 ----


// ---- DB アクセス（イベント取得）----


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

// ---- DB アクセス（範囲内イベント取得）----

func fetchEventsInBounds(ctx context.Context, north, south, east, west float64) ([]Event, error) {
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, fmt.Errorf("DB接続失敗: %w", err)
	}
	defer conn.Close(ctx)

	// PostGISのST_Contains関数を使用して範囲内のイベントを取得
	// 今日開催中のイベントで、かつ指定された範囲内にあるものを取得
	jst := mustJST()
	today := time.Now().In(jst).Format("2006-01-02")
	
	q := `
		SELECT id, title, start_date, end_date, ST_AsText(lnglat), location, category, site_url, icon_category
		FROM events
		WHERE start_date <= $1::date AND end_date >= $1::date
		  AND ST_X(lnglat::geometry) BETWEEN $2 AND $3
		  AND ST_Y(lnglat::geometry) BETWEEN $4 AND $5
		ORDER BY start_date, title
	`
	
	rows, err := conn.Query(ctx, q, today, west, east, south, north)
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