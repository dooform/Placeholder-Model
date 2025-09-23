package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/starwalkn/gotenberg-go-client/v8/document"
)

type PDFService struct {
	client  *gotenberg.Client
	timeout time.Duration
}

func NewPDFService(gotenbergURL string, timeoutStr string) (*PDFService, error) {
	// Parse timeout from configuration
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		// Fallback to optimized default if parsing fails
		timeout = 30 * time.Second // Reduced for faster processing
		fmt.Printf("Warning: failed to parse timeout '%s', using optimized default 30s: %v\n", timeoutStr, err)
	}

	// Create HTTP client with the configured timeout
	httpClient := &http.Client{
		Timeout: timeout,
	}

	client, err := gotenberg.NewClient(gotenbergURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gotenberg client: %w", err)
	}

	return &PDFService{
		client:  client,
		timeout: timeout,
	}, nil
}

func (s *PDFService) ConvertDocxToPDF(ctx context.Context, docxReader io.Reader, filename string) (io.ReadCloser, error) {
	return s.ConvertDocxToPDFWithOrientation(ctx, docxReader, filename, false)
}

func (s *PDFService) ConvertDocxToPDFWithOrientation(ctx context.Context, docxReader io.Reader, filename string, landscape bool) (io.ReadCloser, error) {
	return s.convertWithRetry(ctx, docxReader, filename, landscape, 3)
}

func (s *PDFService) convertWithRetry(ctx context.Context, docxReader io.Reader, filename string, landscape bool, maxRetries int) (io.ReadCloser, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a new context with extended timeout for each attempt
		convertCtx, cancel := context.WithTimeout(ctx, s.timeout)
		defer cancel()

		// Create document from reader
		doc, err := document.FromReader(filename, docxReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create document from reader: %w", err)
		}

		// Create LibreOffice request for DOCX conversion
		req := gotenberg.NewLibreOfficeRequest(doc)

		// Set orientation based on parameter
		if landscape {
			req.Landscape()
		}
		// If landscape is false, default to portrait (don't call Landscape())

		// Convert document
		resp, err := s.client.Send(convertCtx, req)
		if err == nil {
			return resp.Body, nil
		}

		lastErr = err
		fmt.Printf("PDF conversion attempt %d/%d failed: %v\n", attempt, maxRetries, err)

		// If this is not the last attempt, wait before retrying
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed to convert document after %d attempts: %w", maxRetries, lastErr)
}

func (s *PDFService) ConvertDocxToPDFFromFile(ctx context.Context, docxFilePath string) (io.ReadCloser, error) {
	// Create document from file path
	doc, err := document.FromPath("document.docx", docxFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create document from path: %w", err)
	}

	// Create LibreOffice request for DOCX conversion
	req := gotenberg.NewLibreOfficeRequest(doc)

	// Default to portrait orientation

	// Convert document
	resp, err := s.client.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert document: %w", err)
	}

	return resp.Body, nil
}

func (s *PDFService) ConvertDocxToPDFToFile(ctx context.Context, docxReader io.Reader, filename string, outputPath string) error {
	return s.ConvertDocxToPDFToFileWithOrientation(ctx, docxReader, filename, outputPath, false)
}

func (s *PDFService) ConvertDocxToPDFToFileWithOrientation(ctx context.Context, docxReader io.Reader, filename string, outputPath string, landscape bool) error {
	// Create document from reader
	doc, err := document.FromReader(filename, docxReader)
	if err != nil {
		return fmt.Errorf("failed to create document from reader: %w", err)
	}

	// Create LibreOffice request for DOCX conversion
	req := gotenberg.NewLibreOfficeRequest(doc)

	// Set orientation based on parameter
	if landscape {
		req.Landscape()
	}
	// If landscape is false, default to portrait (don't call Landscape())

	// Store the result to file
	err = s.client.Store(ctx, req, outputPath)
	if err != nil {
		return fmt.Errorf("failed to store converted document: %w", err)
	}

	return nil
}

func (s *PDFService) GetClient() *gotenberg.Client {
	return s.client
}

func (s *PDFService) Close() error {
	// Gotenberg client doesn't need explicit closing
	// The HTTP client will be cleaned up automatically
	return nil
}