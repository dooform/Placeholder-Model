package handlers

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

type FileCleanupService struct {
	uploadDir  string
	outputDir  string
	maxAge     time.Duration
	ticker     *time.Ticker
	done       chan bool
}

func NewFileCleanupService(uploadDir, outputDir string, maxAge time.Duration) *FileCleanupService {
	return &FileCleanupService{
		uploadDir: uploadDir,
		outputDir: outputDir,
		maxAge:    maxAge,
		done:      make(chan bool),
	}
}

func (fcs *FileCleanupService) Start() {
	fcs.ticker = time.NewTicker(1 * time.Hour) // Run cleanup every hour
	go func() {
		for {
			select {
			case <-fcs.done:
				return
			case <-fcs.ticker.C:
				fcs.cleanupOldFiles()
			}
		}
	}()
	log.Println("File cleanup service started")
}

func (fcs *FileCleanupService) Stop() {
	if fcs.ticker != nil {
		fcs.ticker.Stop()
	}
	fcs.done <- true
	log.Println("File cleanup service stopped")
}

func (fcs *FileCleanupService) cleanupOldFiles() {
	fcs.cleanupDirectory(fcs.uploadDir)
	fcs.cleanupDirectory(fcs.outputDir)
}

func (fcs *FileCleanupService) cleanupDirectory(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && time.Since(info.ModTime()) > fcs.maxAge {
			log.Printf("Cleaning up old file: %s", path)
			return os.Remove(path)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error during cleanup of %s: %v", dir, err)
	}
}

// DeleteFile manually deletes a specific file
func (fcs *FileCleanupService) DeleteFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to delete
	}

	log.Printf("Manually deleting file: %s", filePath)
	return os.Remove(filePath)
}