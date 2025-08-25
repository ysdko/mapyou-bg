package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)


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

func getEventsBoundsHandler(c *gin.Context) {
	fmt.Println("getEventsBoundsHandler called")
	ctx := c.Request.Context()
	
	// クエリパラメータから境界座標を取得
	northStr := c.Query("north")
	southStr := c.Query("south")
	eastStr := c.Query("east")
	westStr := c.Query("west")
	fieldsStr := c.Query("fields") // 軽量データオプション
	
	if northStr == "" || southStr == "" || eastStr == "" || westStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required parameters: north, south, east, west"})
		return
	}
	
	north, err := strconv.ParseFloat(northStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid north parameter"})
		return
	}
	
	south, err := strconv.ParseFloat(southStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid south parameter"})
		return
	}
	
	east, err := strconv.ParseFloat(eastStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid east parameter"})
		return
	}
	
	west, err := strconv.ParseFloat(westStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid west parameter"})
		return
	}
	
	fmt.Printf("Fetching events in bounds: N=%f, S=%f, E=%f, W=%f, fields=%s\n", north, south, east, west, fieldsStr)
	
	// fieldsパラメータがある場合は軽量データのみを取得
	isLightweight := fieldsStr != ""
	
	events, err := fetchEventsInBounds(ctx, north, south, east, west, isLightweight)
	if err != nil {
		fmt.Printf("Error fetching events in bounds: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, events)
}

// 個別のイベント詳細を取得するハンドラー
func getEventDetailsHandler(c *gin.Context) {
	fmt.Println("getEventDetailsHandler called")
	ctx := c.Request.Context()
	
	// パラメータからevent_idを取得
	eventIDStr := c.Param("event_id")
	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event_id"})
		return
	}
	
	fmt.Printf("Fetching event details for ID: %d\n", eventID)
	
	event, err := fetchEventDetails(ctx, eventID)
	if err != nil {
		fmt.Printf("Error fetching event details: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	
	c.JSON(http.StatusOK, event)
}

