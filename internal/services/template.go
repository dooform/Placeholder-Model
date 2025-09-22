package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"

	"DF-PLCH/internal"
	"DF-PLCH/internal/models"
	"DF-PLCH/internal/processor"
	"DF-PLCH/internal/storage"

	"github.com/google/uuid"
)

type TemplateService struct {
	gcsClient *storage.GCSClient
}

func NewTemplateService(gcsClient *storage.GCSClient) *TemplateService {
	return &TemplateService{
		gcsClient: gcsClient,
	}
}

func (s *TemplateService) UploadTemplate(ctx context.Context, file multipart.File, header *multipart.FileHeader) (*models.Template, error) {
	templateID := uuid.New().String()
	objectName := storage.GenerateObjectName(templateID, header.Filename)

	// Upload to GCS
	result, err := s.gcsClient.UploadFile(ctx, file, objectName, header.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to upload to GCS: %w", err)
	}

	// Create temp file for processing
	file.Seek(0, 0) // Reset file pointer
	tempFile, err := s.createTempFile(file)
	if err != nil {
		// Cleanup GCS file on failure
		s.gcsClient.DeleteFile(ctx, objectName)
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer s.cleanupTempFile(tempFile)

	// Process DOCX to extract placeholders
	proc := processor.NewDocxProcessor(tempFile, "")
	if err := proc.UnzipDocx(); err != nil {
		s.gcsClient.DeleteFile(ctx, objectName)
		return nil, fmt.Errorf("failed to process document: %w", err)
	}
	defer proc.Cleanup()

	placeholders, err := proc.ExtractPlaceholders()
	if err != nil {
		s.gcsClient.DeleteFile(ctx, objectName)
		return nil, fmt.Errorf("failed to extract placeholders: %w", err)
	}

	// Convert placeholders to JSON
	placeholdersJSON, err := json.Marshal(placeholders)
	if err != nil {
		s.gcsClient.DeleteFile(ctx, objectName)
		return nil, fmt.Errorf("failed to marshal placeholders: %w", err)
	}

	// Save to database
	template := &models.Template{
		ID:           templateID,
		Filename:     header.Filename,
		OriginalName: header.Filename,
		GCSPath:      objectName,
		FileSize:     result.Size,
		MimeType:     header.Header.Get("Content-Type"),
		Placeholders: string(placeholdersJSON),
	}

	if err := internal.DB.Create(template).Error; err != nil {
		s.gcsClient.DeleteFile(ctx, objectName)
		return nil, fmt.Errorf("failed to save template metadata: %w", err)
	}

	return template, nil
}

func (s *TemplateService) GetTemplate(templateID string) (*models.Template, error) {
	var template models.Template
	if err := internal.DB.First(&template, "id = ?", templateID).Error; err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	return &template, nil
}

func (s *TemplateService) GetPlaceholders(templateID string) ([]string, error) {
	template, err := s.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	var placeholders []string
	if err := json.Unmarshal([]byte(template.Placeholders), &placeholders); err != nil {
		return nil, fmt.Errorf("failed to unmarshal placeholders: %w", err)
	}

	return placeholders, nil
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, templateID string) error {
	template, err := s.GetTemplate(templateID)
	if err != nil {
		return err
	}

	// Delete from GCS
	if err := s.gcsClient.DeleteFile(ctx, template.GCSPath); err != nil {
		// Log error but continue with database deletion
		fmt.Printf("Warning: failed to delete GCS file %s: %v\n", template.GCSPath, err)
	}

	// Soft delete from database
	return internal.DB.Delete(template).Error
}

func (s *TemplateService) createTempFile(reader io.Reader) (string, error) {
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

func (s *TemplateService) cleanupTempFile(filePath string) {
	os.Remove(filePath)
}