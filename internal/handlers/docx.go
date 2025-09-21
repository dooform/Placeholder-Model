package handlers

import (
	"net/http"

	"DF-PLCH/internal/processor"
	"github.com/gin-gonic/gin"
)

type PlaceholderResponse struct {
	Placeholders []string `json:"placeholders"`
}

type ProcessRequest struct {
	Data map[string]string `json:"data"`
}

func GetPlaceholders(c *gin.Context) {
	inputFile := "../../example.docx"
	proc := processor.NewDocxProcessor(inputFile, "")

	if err := proc.UnzipDocx(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process document"})
		return
	}
	defer proc.Cleanup()

	placeholders, err := proc.ExtractPlaceholders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract placeholders"})
		return
	}

	response := PlaceholderResponse{
		Placeholders: placeholders,
	}

	c.JSON(http.StatusOK, response)
}

func ProcessDocument(c *gin.Context) {
	var req ProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	inputFile := "../../example.docx"
	outputFile := "../../output_modified.docx"
	proc := processor.NewDocxProcessor(inputFile, outputFile)

	if err := proc.UnzipDocx(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unzip document"})
		return
	}
	defer proc.Cleanup()

	placeholders, err := proc.ExtractPlaceholders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract placeholders"})
		return
	}

	completeData := make(map[string]string)
	for _, placeholder := range placeholders {
		if value, exists := req.Data[placeholder]; exists {
			completeData[placeholder] = value
		} else {
			completeData[placeholder] = ""
		}
	}

	if err := proc.FindAndReplaceInDocument(completeData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to replace placeholders"})
		return
	}

	if err := proc.ReZipDocx(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create output document"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Document processed successfully",
		"output_file": outputFile,
	})
}