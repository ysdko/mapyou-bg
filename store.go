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

// マップ表示用の軽量なイベント構造体
type LightweightEvent struct {
	ID           int     `json:"id"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	IconCategory int     `json:"icon_category"`
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
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
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

// 期間に応じた日付フィルター条件を生成
func getPeriodFilter(period string) (string, []interface{}) {
	jst := mustJST()
	today := time.Now().In(jst)
	
	switch period {
	case "today":
		todayStr := today.Format("2006-01-02")
		return "start_date <= $1::date AND end_date >= $1::date", []interface{}{todayStr}
		
	case "weekend":
		// 今週の土曜日と日曜日を計算
		weekday := int(today.Weekday())
		var saturday, sunday time.Time
		
		if weekday == 0 { // 日曜日
			saturday = today.AddDate(0, 0, -1)
			sunday = today
		} else if weekday == 6 { // 土曜日
			saturday = today
			sunday = today.AddDate(0, 0, 1)
		} else { // 月曜〜金曜
			daysUntilSaturday := 6 - weekday
			saturday = today.AddDate(0, 0, daysUntilSaturday)
			sunday = saturday.AddDate(0, 0, 1)
		}
		
		saturdayStr := saturday.Format("2006-01-02")
		sundayStr := sunday.Format("2006-01-02")
		return "(start_date <= $1::date AND end_date >= $1::date) OR (start_date <= $2::date AND end_date >= $2::date)", []interface{}{saturdayStr, sundayStr}
		
	case "all":
		// 制限なし（ただし、古いイベントは除外）
		oneMonthAgo := today.AddDate(0, -1, 0).Format("2006-01-02")
		return "end_date >= $1::date", []interface{}{oneMonthAgo}
		
	default:
		// デフォルトは今日
		todayStr := today.Format("2006-01-02")
		return "start_date <= $1::date AND end_date >= $1::date", []interface{}{todayStr}
	}
}

func fetchEventsInBounds(ctx context.Context, north, south, east, west float64, isLightweight bool, period string) (interface{}, error) {
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, fmt.Errorf("DB接続失敗: %w", err)
	}
	defer conn.Close(ctx)

	// 期間フィルター条件を取得
	periodFilter, periodArgs := getPeriodFilter(period)
	
	if isLightweight {
		// 軽量データ（マップ表示用）
		q := fmt.Sprintf(`
			SELECT id, ST_AsText(lnglat), icon_category
			FROM events
			WHERE (%s)
			  AND ST_X(lnglat::geometry) BETWEEN $%d AND $%d
			  AND ST_Y(lnglat::geometry) BETWEEN $%d AND $%d
			ORDER BY start_date, title
		`, periodFilter, len(periodArgs)+1, len(periodArgs)+2, len(periodArgs)+3, len(periodArgs)+4)
		
		// パラメータを組み合わせ
		queryArgs := append(periodArgs, west, east, south, north)
		rows, err := conn.Query(ctx, q, queryArgs...)
		if err != nil {
			return nil, fmt.Errorf("クエリ失敗: %w", err)
		}
		defer rows.Close()

		var results []LightweightEvent
		for rows.Next() {
			var e LightweightEvent
			var point string
			if err := rows.Scan(&e.ID, &point, &e.IconCategory); err != nil {
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
	} else {
		// 完全データ（従来の形式）
		q := fmt.Sprintf(`
			SELECT id, title, start_date, end_date, ST_AsText(lnglat), location, category, site_url, icon_category
			FROM events
			WHERE (%s)
			  AND ST_X(lnglat::geometry) BETWEEN $%d AND $%d
			  AND ST_Y(lnglat::geometry) BETWEEN $%d AND $%d
			ORDER BY start_date, title
		`, periodFilter, len(periodArgs)+1, len(periodArgs)+2, len(periodArgs)+3, len(periodArgs)+4)
		
		// パラメータを組み合わせ
		queryArgs := append(periodArgs, west, east, south, north)
		rows, err := conn.Query(ctx, q, queryArgs...)
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
}

// 個別のイベント詳細を取得
func fetchEventDetails(ctx context.Context, eventID int) (*Event, error) {
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, fmt.Errorf("DB接続失敗: %w", err)
	}
	defer conn.Close(ctx)

	q := `
		SELECT id, title, start_date, end_date, ST_AsText(lnglat), location, category, site_url, icon_category
		FROM events
		WHERE id = $1
	`
	
	row := conn.QueryRow(ctx, q, eventID)
	
	var e Event
	var point string
	err = row.Scan(&e.ID, &e.Title, &e.StartDate, &e.EndDate, &point, &e.Location, &e.Category, &e.SiteUrl, &e.IconCategory)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // イベントが見つからない場合
		}
		return nil, fmt.Errorf("クエリ失敗: %w", err)
	}
	
	// POINT(lng lat) → e.Lng / e.Lat
	point = strings.TrimPrefix(point, "POINT(")
	point = strings.TrimSuffix(point, ")")
	parts := strings.Split(point, " ")
	if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%f", &e.Lng)
		fmt.Sscanf(parts[1], "%f", &e.Lat)
	}
	
	return &e, nil
}