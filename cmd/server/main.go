package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"DF-PLCH/internal/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Initialize file cleanup service (files older than 24 hours will be deleted)
	cleanupService := handlers.NewFileCleanupService("uploads", "outputs", 24*time.Hour)
	cleanupService.Start()

	// Graceful shutdown handling
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down server...")
		cleanupService.Stop()
		os.Exit(0)
	}()

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// File upload and template management
		v1.POST("/upload", handlers.UploadTemplate)
		v1.GET("/templates/:templateId/placeholders", handlers.GetPlaceholders)

		// Document processing and download
		v1.POST("/templates/:templateId/process", handlers.ProcessDocument)
		v1.GET("/documents/:documentId/download", handlers.DownloadDocument)
	}

	log.Println("Starting server on :8080")
	r.Run(":8080")
}
