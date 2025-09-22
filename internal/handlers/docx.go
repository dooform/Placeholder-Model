package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"DF-PLCH/internal/processor"
	"DF-PLCH/internal/services"

	"github.com/gin-gonic/gin"
)

type DocxHandler struct {
	templateService *services.TemplateService
	documentService *services.DocumentService
}

func NewDocxHandler(templateService *services.TemplateService, documentService *services.DocumentService) *DocxHandler {
	return &DocxHandler{
		templateService: templateService,
		documentService: documentService,
	}
}

type PlaceholderResponse struct {
	Placeholders []string `json:"placeholders"`
}

type PlaceholderPositionResponse struct {
	Placeholders []processor.PlaceholderPosition `json:"placeholders"`
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
	DocumentID  string `json:"document_id"`
	DownloadURL string `json:"download_url"`
	ExpiresAt   string `json:"expires_at"`
	Message     string `json:"message"`
}

func (h *DocxHandler) UploadTemplate(c *gin.Context) {
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

	template, err := h.templateService.UploadTemplate(c.Request.Context(), file, header)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload template: %v", err)})
		return
	}

	// Parse placeholders from JSON
	var placeholders []string
	if err := json.Unmarshal([]byte(template.Placeholders), &placeholders); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse placeholders"})
		return
	}

	response := UploadResponse{
		TemplateID:   template.ID,
		Placeholders: placeholders,
		Message:      "Template uploaded successfully",
	}

	c.JSON(http.StatusOK, response)
}

func (h *DocxHandler) GetPlaceholders(c *gin.Context) {
	templateID := c.Param("templateId")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template ID is required"})
		return
	}

	placeholders, err := h.templateService.GetPlaceholders(templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	response := PlaceholderResponse{
		Placeholders: placeholders,
	}

	c.JSON(http.StatusOK, response)
}

func (h *DocxHandler) GetPlaceholderPositions(c *gin.Context) {
	templateID := c.Param("templateId")
	if templateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template ID is required"})
		return
	}

	positions, err := h.templateService.GetPlaceholderPositions(templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	response := PlaceholderPositionResponse{
		Placeholders: positions,
	}

	c.JSON(http.StatusOK, response)
}

func (h *DocxHandler) ProcessDocument(c *gin.Context) {
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

	document, err := h.documentService.ProcessDocument(c.Request.Context(), templateID, req.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process document: %v", err)})
		return
	}

	// Create temporary download link that expires in 24 hours
	expiresAt := time.Now().Add(24 * time.Hour)
	response := ProcessResponse{
		DocumentID:  document.ID,
		DownloadURL: fmt.Sprintf("/api/v1/documents/%s/download", document.ID),
		ExpiresAt:   expiresAt.Format(time.RFC3339),
		Message:     "Document processed successfully",
	}

	c.JSON(http.StatusOK, response)
}

func (h *DocxHandler) DownloadDocument(c *gin.Context) {
	documentID := c.Param("documentId")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document ID is required"})
		return
	}

	format := c.DefaultQuery("format", "docx")

	reader, filename, mimeType, err := h.documentService.GetDocumentReader(c.Request.Context(), documentID, format)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}
	defer reader.Close()

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", mimeType)

	// Stream the file to the client
	_, err = io.Copy(c.Writer, reader)
	if err != nil {
		// If streaming fails, don't delete the file
		fmt.Printf("Error streaming file: %v\n", err)
		return
	}

	// After successful download, delete the processed DOCX file from GCS
	// but keep the document record in database with user data
	go func() {
		if err := h.documentService.DeleteProcessedFile(c.Request.Context(), documentID, format); err != nil {
			fmt.Printf("Warning: failed to delete processed file for document %s: %v\n", documentID, err)
		}
	}()
}

// Legacy functions for backward compatibility - these will be removed
func UploadTemplate(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "This endpoint is deprecated. Use dependency injection instead."})
}

func GetPlaceholders(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "This endpoint is deprecated. Use dependency injection instead."})
}

func ProcessDocument(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "This endpoint is deprecated. Use dependency injection instead."})
}

func DownloadDocument(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "This endpoint is deprecated. Use dependency injection instead."})
}
