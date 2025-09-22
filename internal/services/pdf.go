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

func NewPDFService(gotenbergURL string) (*PDFService, error) {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client, err := gotenberg.NewClient(gotenbergURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gotenberg client: %w", err)
	}

	return &PDFService{
		client:  client,
		timeout: 30 * time.Second,
	}, nil
}

func (s *PDFService) ConvertDocxToPDF(ctx context.Context, docxReader io.Reader, filename string) (io.ReadCloser, error) {
	// Create document from reader
	doc, err := document.FromReader(filename, docxReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create document from reader: %w", err)
	}

	// Create LibreOffice request for DOCX conversion
	req := gotenberg.NewLibreOfficeRequest(doc)

	// Set landscape mode to false (portrait)
	req.Landscape()

	// Convert document
	resp, err := s.client.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert document: %w", err)
	}

	return resp.Body, nil
}

func (s *PDFService) ConvertDocxToPDFFromFile(ctx context.Context, docxFilePath string) (io.ReadCloser, error) {
	// Create document from file path
	doc, err := document.FromPath("document.docx", docxFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create document from path: %w", err)
	}

	// Create LibreOffice request for DOCX conversion
	req := gotenberg.NewLibreOfficeRequest(doc)

	// Convert document
	resp, err := s.client.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert document: %w", err)
	}

	return resp.Body, nil
}

func (s *PDFService) ConvertDocxToPDFToFile(ctx context.Context, docxReader io.Reader, filename string, outputPath string) error {
	// Create document from reader
	doc, err := document.FromReader(filename, docxReader)
	if err != nil {
		return fmt.Errorf("failed to create document from reader: %w", err)
	}

	// Create LibreOffice request for DOCX conversion
	req := gotenberg.NewLibreOfficeRequest(doc)

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