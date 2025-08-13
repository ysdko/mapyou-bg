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

	// 毎日 00:01 JST に実行
	go func() {
		for {
			sleepUntil := nextJSTMidnightPlus(time.Now(), 1)
			time.Sleep(time.Until(sleepUntil))
			if err := ensureTodayFileAndCleanup(); err != nil {
				log.Printf("daily ensure failed: %v", err)
			}
		}
	}()

	// ルータ
	r := gin.Default()
	r.Use(cors.Default())

	// ルート
	r.GET("/events/today", getTodayEventsHandler)

	log.Println("server :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
