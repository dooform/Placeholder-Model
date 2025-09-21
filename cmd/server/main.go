package main

import (
	"log"

	"DF-PLCH/internal/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	r.GET("/placeholders", handlers.GetPlaceholders)
	r.POST("/process", handlers.ProcessDocument)

	log.Println("Starting server on :8080")
	r.Run(":8080")
}
