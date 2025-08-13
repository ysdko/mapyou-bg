package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func getTodayEventsHandler(c *gin.Context) {
	jst := mustJST()
	today := time.Now().In(jst)
	path := jsonPathForDate(today)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := writeJsonForDateFromDB(today); err != nil {
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

