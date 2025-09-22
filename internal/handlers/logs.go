package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"DF-PLCH/internal/models"
	"DF-PLCH/internal/services"

	"github.com/gin-gonic/gin"
)

type LogsHandler struct {
	activityLogService *services.ActivityLogService
}

func NewLogsHandler(activityLogService *services.ActivityLogService) *LogsHandler {
	return &LogsHandler{
		activityLogService: activityLogService,
	}
}

type LogsResponse struct {
	Logs       interface{} `json:"logs"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

// GetAllLogs returns all activity logs with pagination
func (h *LogsHandler) GetAllLogs(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	pageStr := c.DefaultQuery("page", "1")
	method := c.Query("method")
	path := c.Query("path")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 1000 { // Prevent too large requests
		limit = 1000
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	offset := (page - 1) * limit

	var logs []models.ActivityLog
	var total int64

	// Filter by method if provided
	if method != "" {
		logs, total, err = h.activityLogService.GetLogsByMethod(method, limit, offset)
	} else if path != "" {
		// Filter by path if provided
		logs, total, err = h.activityLogService.GetLogsByPath(path, limit, offset)
	} else {
		// Get all logs
		logs, total, err = h.activityLogService.GetAllLogs(limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch logs"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	response := LogsResponse{
		Logs:       logs,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// GetLogStats returns statistics about the logs
func (h *LogsHandler) GetLogStats(c *gin.Context) {
	// This could be extended to provide more detailed statistics
	logs, total, err := h.activityLogService.GetAllLogs(0, 0) // Get all logs
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch log stats"})
		return
	}

	// Count by method
	methodCounts := make(map[string]int)
	pathCounts := make(map[string]int)
	statusCounts := make(map[int]int)

	for _, log := range logs {
		methodCounts[log.Method]++
		pathCounts[log.Path]++
		statusCounts[log.StatusCode]++
	}

	stats := gin.H{
		"total_requests": total,
		"methods":        methodCounts,
		"paths":          pathCounts,
		"status_codes":   statusCounts,
	}

	c.JSON(http.StatusOK, stats)
}

// GetProcessLogs returns only logs from the process endpoint with data users sent
func (h *LogsHandler) GetProcessLogs(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	pageStr := c.DefaultQuery("page", "1")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	offset := (page - 1) * limit

	// Filter by process path specifically
	logs, total, err := h.activityLogService.GetLogsByPath("process", limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch process logs"})
		return
	}

	// Extract just the data from process requests
	processData := make([]gin.H, 0)
	for _, log := range logs {
		if log.Method == "POST" && len(log.RequestBody) > 0 {
			// Try to parse the JSON data from request body
			var requestData map[string]interface{}
			if err := json.Unmarshal([]byte(log.RequestBody), &requestData); err == nil {
				processData = append(processData, gin.H{
					"timestamp":    log.CreatedAt,
					"template_id":  extractTemplateID(log.Path),
					"user_data":    requestData,
					"ip_address":   log.IPAddress,
					"user_agent":   log.UserAgent,
					"response_time": log.ResponseTime,
				})
			} else {
				// If JSON parsing fails, just show the raw body
				processData = append(processData, gin.H{
					"timestamp":    log.CreatedAt,
					"template_id":  extractTemplateID(log.Path),
					"raw_data":     log.RequestBody,
					"ip_address":   log.IPAddress,
					"user_agent":   log.UserAgent,
					"response_time": log.ResponseTime,
				})
			}
		}
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	response := gin.H{
		"process_data": processData,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"total_pages":  totalPages,
		"description":  "Data sent by users to process endpoints",
	}

	c.JSON(http.StatusOK, response)
}

// Helper function to extract template ID from path like "/api/v1/templates/123/process"
func extractTemplateID(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "templates" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "unknown"
}

// GetAllLogsDebug returns all logs without filtering for debugging
func (h *LogsHandler) GetAllLogsDebug(c *gin.Context) {
	logs, total, err := h.activityLogService.GetAllLogs(20, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
		"debug": "All logs for debugging",
	})
}

// GetHistory returns only POST requests with user data
func (h *LogsHandler) GetHistory(c *gin.Context) {
	// Get all logs
	logs, _, err := h.activityLogService.GetAllLogs(100, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch logs"})
		return
	}

	// Filter only POST requests with bodies and format them nicely
	history := make([]gin.H, 0)
	for _, log := range logs {
		if log.Method == "POST" && len(log.RequestBody) > 0 {
			// Try to parse JSON body
			var userData interface{}
			if err := json.Unmarshal([]byte(log.RequestBody), &userData); err == nil {
				history = append(history, gin.H{
					"timestamp":    log.CreatedAt,
					"path":         log.Path,
					"template_id":  extractTemplateID(log.Path),
					"user_data":    userData,
					"ip_address":   log.IPAddress,
					"user_agent":   log.UserAgent,
					"response_time": log.ResponseTime,
				})
			} else {
				// If not JSON, show raw body
				history = append(history, gin.H{
					"timestamp":    log.CreatedAt,
					"path":         log.Path,
					"template_id":  extractTemplateID(log.Path),
					"raw_body":     log.RequestBody,
					"ip_address":   log.IPAddress,
					"user_agent":   log.UserAgent,
					"response_time": log.ResponseTime,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   len(history),
		"message": "All POST requests with user data",
	})
}