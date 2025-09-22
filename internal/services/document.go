package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"DF-PLCH/internal"
	"DF-PLCH/internal/models"
	"DF-PLCH/internal/processor"
	"DF-PLCH/internal/storage"

	"github.com/google/uuid"
)

type DocumentService struct {
	gcsClient       *storage.GCSClient
	templateService *TemplateService
}

func NewDocumentService(gcsClient *storage.GCSClient, templateService *TemplateService) *DocumentService {
	return &DocumentService{
		gcsClient:       gcsClient,
		templateService: templateService,
	}
}

func (s *DocumentService) ProcessDocument(ctx context.Context, templateID string, data map[string]string) (*models.Document, error) {
	// Get template
	template, err := s.templateService.GetTemplate(templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Download template from GCS
	reader, err := s.gcsClient.ReadFile(ctx, template.GCSPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template from GCS: %w", err)
	}
	defer reader.Close()

	// Create temp input file
	tempInputFile, err := s.createTempFile(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer s.cleanupTempFile(tempInputFile)

	// Create temp output file
	documentID := uuid.New().String()
	tempOutputFile := filepath.Join(os.TempDir(), documentID+".docx")

	// Process document
	proc := processor.NewDocxProcessor(tempInputFile, tempOutputFile)
	if err := proc.UnzipDocx(); err != nil {
		return nil, fmt.Errorf("failed to unzip document: %w", err)
	}
	defer proc.Cleanup()

	// Get placeholders and prepare complete data
	placeholders, err := proc.ExtractPlaceholders()
	if err != nil {
		return nil, fmt.Errorf("failed to extract placeholders: %w", err)
	}

	completeData := make(map[string]string)
	for _, placeholder := range placeholders {
		if value, exists := data[placeholder]; exists {
			completeData[placeholder] = value
		} else {
			completeData[placeholder] = ""
		}
	}

	// Replace placeholders
	if err := proc.FindAndReplaceInDocument(completeData); err != nil {
		return nil, fmt.Errorf("failed to replace placeholders: %w", err)
	}

	// Re-zip document
	if err := proc.ReZipDocx(); err != nil {
		return nil, fmt.Errorf("failed to create output document: %w", err)
	}

	// Upload processed document to GCS
	outputFile, err := os.Open(tempOutputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open output file: %w", err)
	}
	defer outputFile.Close()
	defer os.Remove(tempOutputFile)

	objectName := storage.GenerateDocumentObjectName(documentID, template.Filename)
	result, err := s.gcsClient.UploadFile(ctx, outputFile, objectName, "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	if err != nil {
		return nil, fmt.Errorf("failed to upload processed document to GCS: %w", err)
	}

	// Convert data to JSON
	dataJSON, err := json.Marshal(completeData)
	if err != nil {
		s.gcsClient.DeleteFile(ctx, objectName)
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	// Save document metadata
	document := &models.Document{
		ID:         documentID,
		TemplateID: templateID,
		Filename:   template.Filename,
		GCSPath:    objectName,
		FileSize:   result.Size,
		MimeType:   "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Data:       string(dataJSON),
		Status:     "completed",
	}

	if err := internal.DB.Create(document).Error; err != nil {
		s.gcsClient.DeleteFile(ctx, objectName)
		return nil, fmt.Errorf("failed to save document metadata: %w", err)
	}

	return document, nil
}

func (s *DocumentService) GetDocument(documentID string) (*models.Document, error) {
	var document models.Document
	if err := internal.DB.First(&document, "id = ?", documentID).Error; err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}
	return &document, nil
}

func (s *DocumentService) GetDocumentReader(ctx context.Context, documentID string) (io.ReadCloser, string, error) {
	document, err := s.GetDocument(documentID)
	if err != nil {
		return nil, "", err
	}

	reader, err := s.gcsClient.ReadFile(ctx, document.GCSPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read document from GCS: %w", err)
	}

	return reader, document.Filename, nil
}

func (s *DocumentService) DeleteDocument(ctx context.Context, documentID string) error {
	document, err := s.GetDocument(documentID)
	if err != nil {
		return err
	}

	// Delete from GCS
	if err := s.gcsClient.DeleteFile(ctx, document.GCSPath); err != nil {
		// Log error but continue with database deletion
		fmt.Printf("Warning: failed to delete GCS file %s: %v\n", document.GCSPath, err)
	}

	// Soft delete from database
	return internal.DB.Delete(document).Error
}

// DeleteProcessedFile deletes only the processed DOCX file from GCS
// but keeps the document record with user data in the database
func (s *DocumentService) DeleteProcessedFile(ctx context.Context, documentID string) error {
	document, err := s.GetDocument(documentID)
	if err != nil {
		return err
	}

	// Delete only the GCS file, keep database record
	if err := s.gcsClient.DeleteFile(ctx, document.GCSPath); err != nil {
		return fmt.Errorf("failed to delete processed file from GCS: %w", err)
	}

	// Update document status to indicate file has been downloaded and deleted
	if err := internal.DB.Model(document).Update("status", "downloaded").Error; err != nil {
		fmt.Printf("Warning: failed to update document status: %v\n", err)
	}

	return nil
}

func (s *DocumentService) createTempFile(reader io.Reader) (string, error) {
	tempFile, err := os.CreateTemp("", "*.docx")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, reader)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

func (s *DocumentService) cleanupTempFile(filePath string) {
	os.Remove(filePath)
}