package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// .env 読み込み（なくてもOK）
	_ = godotenv.Load(".env")
	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("DATABASE_URL is not set")
	}


	// 起動時に今日のファイルを用意＆古いファイル掃除
	if err := ensureTodayFileAndCleanup(); err != nil {
		log.Printf("initial ensure failed: %v", err)
	}

	// 日付が変わったタイミングで実行
	go func() {
		for {
			sleepUntil := nextJSTMidnightPlus(time.Now(), 1)
			time.Sleep(time.Until(sleepUntil))
			if err := ensureTodayFileAndCleanup(); err != nil {
				log.Printf("daily ensure failed: %v", err)
			}
		}
	}()

	//　CORS 設定
	r := gin.Default()
	allowOrigin := os.Getenv("ALLOWED_ORIGINS")
	cfg := cors.Config{
	  AllowOrigins:     []string{allowOrigin},
	  AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	  AllowHeaders:     []string{"Content-Type", "Authorization"},
	  ExposeHeaders:    []string{"Content-Length"},
	  AllowCredentials: true, 
	  MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(cfg))

	// ルート
	r.GET("/events/today", getTodayEventsHandler)
	r.POST("/reviews", postEventMessageHandler)
	r.GET("/reviews/:event_id", getEventReviewHandler)

	log.Println("server :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
