package processor

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DocxProcessor struct {
	inputFile  string
	outputFile string
	tempDir    string
}

func NewDocxProcessor(inputFile, outputFile string) *DocxProcessor {
	return &DocxProcessor{
		inputFile:  inputFile,
		outputFile: outputFile,
		tempDir:    fmt.Sprintf("temp_docx_%d", time.Now().UnixNano()),
	}
}

func (dp *DocxProcessor) UnzipDocx() error {
	reader, err := zip.OpenReader(dp.inputFile)
	if err != nil {
		return fmt.Errorf("failed to open docx file: %w", err)
	}
	defer reader.Close()

	err = os.MkdirAll(dp.tempDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	for _, file := range reader.File {
		err := dp.extractFile(file)
		if err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	return nil
}

func (dp *DocxProcessor) extractFile(file *zip.File) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	path := filepath.Join(dp.tempDir, file.Name)

	if file.FileInfo().IsDir() {
		os.MkdirAll(path, file.FileInfo().Mode())
		return nil
	}

	os.MkdirAll(filepath.Dir(path), 0755)

	outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

func (dp *DocxProcessor) FindAndReplaceInDocument(placeholders map[string]string) error {
	documentPath := filepath.Join(dp.tempDir, "word", "document.xml")

	content, err := os.ReadFile(documentPath)
	if err != nil {
		return fmt.Errorf("failed to read document.xml: %w", err)
	}

	contentStr := string(content)

	for placeholder, value := range placeholders {
		contentStr = dp.replaceWithXMLHandling(contentStr, placeholder, value)
	}

	err = os.WriteFile(documentPath, []byte(contentStr), 0644)
	if err != nil {
		return fmt.Errorf("failed to write document.xml: %w", err)
	}

	return nil
}

func (dp *DocxProcessor) replaceWithXMLHandling(content, placeholder, value string) string {
	if strings.Contains(content, placeholder) {
		return strings.ReplaceAll(content, placeholder, value)
	}

	placeholderChars := []rune(placeholder)
	result := ""
	i := 0

	for pos := 0; pos < len(content); pos++ {
		char := rune(content[pos])

		if i < len(placeholderChars) && char == placeholderChars[i] {
			match, endPos := dp.checkPlaceholderMatch(content, pos, placeholder)
			if match {
				result += value
				pos = endPos - 1
				i = 0
				continue
			}
		}

		result += string(char)
		i = 0
	}

	return result
}

func (dp *DocxProcessor) checkPlaceholderMatch(content string, startPos int, placeholder string) (bool, int) {
	placeholderChars := []rune(placeholder)
	contentRunes := []rune(content)

	if startPos >= len(contentRunes) {
		return false, startPos
	}

	matchIndex := 0
	pos := startPos
	inTag := false

	for pos < len(contentRunes) && matchIndex < len(placeholderChars) {
		char := contentRunes[pos]

		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			if char == placeholderChars[matchIndex] {
				matchIndex++
			} else {
				return false, startPos
			}
		}

		pos++
	}

	return matchIndex == len(placeholderChars), pos
}

func (dp *DocxProcessor) ReZipDocx() error {
	outputFile, err := os.Create(dp.outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	zipWriter := zip.NewWriter(outputFile)
	defer zipWriter.Close()

	return filepath.Walk(dp.tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dp.tempDir, path)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)

		zipFile, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(zipFile, file)
		return err
	})
}

func (dp *DocxProcessor) Cleanup() {
	os.RemoveAll(dp.tempDir)
}

func (dp *DocxProcessor) ExtractPlaceholders() ([]string, error) {
	documentPath := filepath.Join(dp.tempDir, "word", "document.xml")

	content, err := os.ReadFile(documentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read document.xml: %w", err)
	}

	contentStr := string(content)
	cleanText := dp.removeXMLTags(contentStr)

	var placeholders []string
	start := 0
	for {
		startIndex := strings.Index(cleanText[start:], "{{")
		if startIndex == -1 {
			break
		}
		startIndex += start

		endIndex := strings.Index(cleanText[startIndex:], "}}")
		if endIndex == -1 {
			break
		}
		endIndex += startIndex + 2

		placeholder := cleanText[startIndex:endIndex]

		found := false
		for _, existing := range placeholders {
			if existing == placeholder {
				found = true
				break
			}
		}

		if !found {
			placeholders = append(placeholders, placeholder)
		}

		start = endIndex
	}

	return placeholders, nil
}

func (dp *DocxProcessor) removeXMLTags(content string) string {
	result := ""
	inTag := false

	for _, char := range content {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			result += string(char)
		}
	}

	return result
}

func (dp *DocxProcessor) AutoFillPlaceholders(placeholders []string) map[string]string {
	userInput := make(map[string]string)

	fmt.Printf("\nFound %d unique placeholders in the document:\n", len(placeholders))
	fmt.Println("Auto-filling with test values:")
	fmt.Println("=============================")

	for _, placeholder := range placeholders {
		var value string

		switch {
		case strings.Contains(placeholder, "namePerson1"):
			value = "John Doe"
		case strings.Contains(placeholder, "namePerson2"):
			value = "Jane Smith"
		case strings.Contains(placeholder, "regisOffice"):
			value = "Municipal Office"
		case strings.Contains(placeholder, "Province"):
			value = "Ontario"
		case strings.Contains(placeholder, "registrationNo"):
			value = "REG-2024-001"
		case strings.Contains(placeholder, "date"):
			value = "September 21, 2025"
		case strings.Contains(placeholder, "signName"):
			value = "Dhanavadh Saito"
		case strings.Contains(placeholder, "c_"):
			value = "5"
		case strings.Contains(placeholder, "m_id_"):
			value = "7"
		case strings.Contains(placeholder, "w_id_"):
			value = "3"
		default:
			value = "TEST"
		}

		userInput[placeholder] = value
		fmt.Printf("   %s = '%s'\n", placeholder, value)
	}

	return userInput
}

func (dp *DocxProcessor) Process() error {
	fmt.Println("1. Unzipping docx file...")
	if err := dp.UnzipDocx(); err != nil {
		return err
	}
	defer dp.Cleanup()

	fmt.Println("2. Extracting placeholders from document...")
	placeholders, err := dp.ExtractPlaceholders()
	if err != nil {
		return err
	}

	fmt.Println("3. Auto-filling placeholders with test values...")
	userInput := dp.AutoFillPlaceholders(placeholders)

	fmt.Println("\n4. Replacing placeholders in document...")
	if err := dp.FindAndReplaceInDocument(userInput); err != nil {
		return err
	}

	fmt.Println("5. Re-zipping modified document...")
	if err := dp.ReZipDocx(); err != nil {
		return err
	}

	fmt.Printf("6. Successfully created: %s\n", dp.outputFile)
	return nil
}