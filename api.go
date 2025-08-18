package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func getTodayEventsHandler(c *gin.Context) {
	fmt.Println("getTodayEventsHandler called")
	jst := mustJST()
	today := time.Now().In(jst)
	path := jsonPathForDate(today)

	// ないなら作る（ついでに古いの掃除）
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := writeJsonForDateFromDB(today); err != nil {
			fmt.Printf("Failed to prepare today json: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare today json"})
			return
		}
		_ = cleanupOldFiles(today)
	}

	events, err := readEventsFromFile(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read json"})
		return
	}
	c.JSON(http.StatusOK, events)
}

func postEventMessageHandler(c *gin.Context) {
	// --- デバッグ: Raw Body を出したい場合は最初に読み込む ---
	raw, _ := io.ReadAll(c.Request.Body)
	if len(raw) > 0 {
		fmt.Printf("Raw request body: %s\n", string(raw))
		// 読み尽くしたのでバインド用に戻す
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
	}

	// フロントからの JSON を受け取る（comment に統一）
	var req struct {
		UserID  string `json:"user_id"  binding:"required"`            // DB が INT の場合は int に変更
		EventID int    `json:"event_id" binding:"required"`
		Comment string `json:"comment"  binding:"required"`
		Rating  int    `json:"rating"   binding:"required,min=1,max=5"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("Failed to bind JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	fmt.Printf("Parsed request: %+v\n", req)

	// ---- DBへ保存（Exec版：戻り値の id は受け取らない）----
	if err := insertEventMessage(
		c.Request.Context(),
		req.UserID,
		req.EventID,
		req.Comment,
		req.Rating,
	); err != nil {
		// ここは必要に応じて 400/500 を出し分けてもOK
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 成功レスポンス（作成時刻などを返す）
	c.JSON(http.StatusCreated, gin.H{
		"status":   "ok",
		"user_id":  req.UserID,
		"event_id": req.EventID,
		"comment":  req.Comment,
		"rating":   req.Rating,
		"created":  time.Now().UTC(),
	})
}

// // イベントごとのレビュー取得
func getEventReviewHandler(c *gin.Context) {
	fmt.Println("getEventReviewHandler called")
	ctx := c.Request.Context()
	// パラメータから event_id を取得
	fmt.Println("Fetching reviews for event ID:", c.Param("event_id"))
	eventIDStr := c.Param("event_id")
	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event_id"})
		return
	}

	reviews, err := queryEventReviews(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reviews)
}

