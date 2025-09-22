package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"DF-PLCH/internal/processor"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PlaceholderResponse struct {
	Placeholders []string `json:"placeholders"`
}

type ProcessRequest struct {
	Data map[string]string `json:"data"`
}

type UploadResponse struct {
	TemplateID   string   `json:"template_id"`
	Placeholders []string `json:"placeholders"`
	Message      string   `json:"message"`
}

type ProcessResponse struct {
	DocumentID string `json:"document_id"`
	Message    string `json:"message"`
}

func UploadTemplate(c *gin.Context) {
	file, header, err := c.Request.FormFile("template")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	if filepath.Ext(header.Filename) != ".docx" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only .docx files are supported"})
		return
	}

	if err := os.MkdirAll("uploads", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	templateID := uuid.New().String()
	filePath := filepath.Join("uploads", templateID+".docx")

	out, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	proc := processor.NewDocxProcessor(filePath, "")
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

	response := UploadResponse{
		TemplateID:   templateID,
		Placeholders: placeholders,
		Message:      "Template uploaded successfully",
	}

	c.JSON(http.StatusOK, response)
}

func GetPlaceholders(c *gin.Context) {
	templateID := c.Param("templateId")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template ID is required"})
		return
	}

	filePath := filepath.Join("uploads", templateID+".docx")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	proc := processor.NewDocxProcessor(filePath, "")
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
	templateID := c.Param("templateId")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template ID is required"})
		return
	}

	var req ProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	inputFile := filepath.Join("uploads", templateID+".docx")
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	if err := os.MkdirAll("outputs", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create output directory"})
		return
	}

	documentID := uuid.New().String()
	outputFile := filepath.Join("outputs", documentID+".docx")
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

	response := ProcessResponse{
		DocumentID: documentID,
		Message:    "Document processed successfully",
	}

	c.JSON(http.StatusOK, response)
}

func DownloadDocument(c *gin.Context) {
	documentID := c.Param("documentId")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document ID is required"})
		return
	}

	filePath := filepath.Join("outputs", documentID+".docx")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.docx", documentID))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")

	c.File(filePath)
}