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

type PlaceholderPosition struct {
	Placeholder   string  `json:"placeholder"`
	StartPos      int     `json:"start_pos"`
	EndPos        int     `json:"end_pos"`
	Line          int     `json:"line"`
	Column        int     `json:"column"`
	XMLStartPos   int     `json:"xml_start_pos"`
	XMLEndPos     int     `json:"xml_end_pos"`
	X             float64 `json:"x"`             // X coordinate in points (1 point = 1/72 inch)
	Y             float64 `json:"y"`             // Y coordinate in points
	Width         float64 `json:"width"`         // Estimated width in points
	Height        float64 `json:"height"`        // Estimated height in points
	PageNumber    int     `json:"page_number"`   // Page number (1-based)
	ParagraphId   string  `json:"paragraph_id"`  // Paragraph identifier
}

type DocumentLayout struct {
	PageWidth    float64 // Page width in points
	PageHeight   float64 // Page height in points
	LeftMargin   float64 // Left margin in points
	RightMargin  float64 // Right margin in points
	TopMargin    float64 // Top margin in points
	BottomMargin float64 // Bottom margin in points
	LineHeight   float64 // Default line height in points
}

type ParagraphInfo struct {
	XMLPosition int     // Position in XML
	LineNumber  int     // Line number in document
	X           float64 // X position
	Y           float64 // Y position
	Width       float64 // Available width
	IsInTable   bool    // Whether paragraph is in a table
	TableRow    int     // Table row if applicable
	TableCol    int     // Table column if applicable
	ParagraphId string  // Paragraph identifier
}

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
	positions, err := dp.ExtractPlaceholdersWithPositions()
	if err != nil {
		return nil, err
	}

	var placeholders []string
	seen := make(map[string]bool)

	for _, pos := range positions {
		if !seen[pos.Placeholder] {
			placeholders = append(placeholders, pos.Placeholder)
			seen[pos.Placeholder] = true
		}
	}

	return placeholders, nil
}

func (dp *DocxProcessor) ExtractPlaceholdersWithPositions() ([]PlaceholderPosition, error) {
	documentPath := filepath.Join(dp.tempDir, "word", "document.xml")

	content, err := os.ReadFile(documentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read document.xml: %w", err)
	}

	contentStr := string(content)
	cleanText := dp.removeXMLTags(contentStr)

	// Parse document layout information
	layout := dp.parseDocumentLayout(contentStr)

	var positions []PlaceholderPosition
	cleanStart := 0

	for {
		startIndex := strings.Index(cleanText[cleanStart:], "{{")
		if startIndex == -1 {
			break
		}
		startIndex += cleanStart

		endIndex := strings.Index(cleanText[startIndex:], "}}")
		if endIndex == -1 {
			break
		}
		endIndex += startIndex + 2

		placeholder := cleanText[startIndex:endIndex]

		// Calculate line and column for start position
		startLine, startColumn := dp.calculateLineColumn(cleanText, startIndex)

		// Find XML positions
		xmlStartPos, xmlEndPos := dp.findXMLPositions(contentStr, cleanText, startIndex, endIndex)

		// Calculate coordinates
		x, y, pageNumber, paragraphId := dp.calculateCoordinates(contentStr, xmlStartPos, layout)

		// Estimate dimensions
		fontSize := 12.0 // Default font size - could be extracted from XML
		width := dp.estimateTextWidth(placeholder, fontSize)
		height := fontSize * 1.2 // Line height

		position := PlaceholderPosition{
			Placeholder: placeholder,
			StartPos:    startIndex,
			EndPos:      endIndex,
			Line:        startLine,
			Column:      startColumn,
			XMLStartPos: xmlStartPos,
			XMLEndPos:   xmlEndPos,
			X:           x,
			Y:           y,
			Width:       width,
			Height:      height,
			PageNumber:  pageNumber,
			ParagraphId: paragraphId,
		}

		positions = append(positions, position)
		cleanStart = endIndex
	}

	return positions, nil
}

func (dp *DocxProcessor) calculateLineColumn(text string, pos int) (int, int) {
	line := 1
	column := 1

	for i := 0; i < pos && i < len(text); i++ {
		if text[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}

	return line, column
}

func (dp *DocxProcessor) findXMLPositions(xmlContent, cleanText string, cleanStart, cleanEnd int) (int, int) {
	// This is a simplified approach - maps clean text positions back to XML
	// For more accuracy, you'd need to track character-by-character mapping during XML removal

	xmlStart := -1
	xmlEnd := -1

	cleanPos := 0
	inTag := false

	for xmlPos, char := range xmlContent {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			if cleanPos == cleanStart && xmlStart == -1 {
				xmlStart = xmlPos
			}
			if cleanPos == cleanEnd-1 {
				xmlEnd = xmlPos + 1
				break
			}
			cleanPos++
		}
	}

	return xmlStart, xmlEnd
}

// Default document layout based on standard A4 page with normal margins
func (dp *DocxProcessor) getDefaultLayout() DocumentLayout {
	return DocumentLayout{
		PageWidth:    612,  // 8.5 inches * 72 points/inch (A4 width)
		PageHeight:   792,  // 11 inches * 72 points/inch (A4 height)
		LeftMargin:   72,   // 1 inch
		RightMargin:  72,   // 1 inch
		TopMargin:    72,   // 1 inch
		BottomMargin: 72,   // 1 inch
		LineHeight:   14.4, // 12pt font with 1.2 line spacing
	}
}

// Parse document layout from document.xml sectPr elements
func (dp *DocxProcessor) parseDocumentLayout(content string) DocumentLayout {
	layout := dp.getDefaultLayout()

	// Look for section properties (w:sectPr)
	if sectStart := strings.Index(content, "<w:sectPr"); sectStart != -1 {
		sectEnd := strings.Index(content[sectStart:], "</w:sectPr>")
		if sectEnd != -1 {
			sectContent := content[sectStart : sectStart+sectEnd]

			// Parse page size (w:pgSz)
			if pgSzStart := strings.Index(sectContent, "<w:pgSz"); pgSzStart != -1 {
				pgSzEnd := strings.Index(sectContent[pgSzStart:], "/>")
				if pgSzEnd != -1 {
					pgSzTag := sectContent[pgSzStart : pgSzStart+pgSzEnd]

					// Extract width and height attributes (in twentieths of a point)
					if wStart := strings.Index(pgSzTag, `w:w="`); wStart != -1 {
						wStart += 5
						wEnd := strings.Index(pgSzTag[wStart:], `"`)
						if wEnd != -1 {
							if width := dp.parseFloatFromTwips(pgSzTag[wStart:wStart+wEnd]); width > 0 {
								layout.PageWidth = width
							}
						}
					}

					if hStart := strings.Index(pgSzTag, `w:h="`); hStart != -1 {
						hStart += 5
						hEnd := strings.Index(pgSzTag[hStart:], `"`)
						if hEnd != -1 {
							if height := dp.parseFloatFromTwips(pgSzTag[hStart:hStart+hEnd]); height > 0 {
								layout.PageHeight = height
							}
						}
					}
				}
			}

			// Parse page margins (w:pgMar)
			if pgMarStart := strings.Index(sectContent, "<w:pgMar"); pgMarStart != -1 {
				pgMarEnd := strings.Index(sectContent[pgMarStart:], "/>")
				if pgMarEnd != -1 {
					pgMarTag := sectContent[pgMarStart : pgMarStart+pgMarEnd]

					margins := map[string]*float64{
						"left":   &layout.LeftMargin,
						"right":  &layout.RightMargin,
						"top":    &layout.TopMargin,
						"bottom": &layout.BottomMargin,
					}

					for attr, ptr := range margins {
						if start := strings.Index(pgMarTag, fmt.Sprintf(`w:%s="`, attr)); start != -1 {
							start += len(attr) + 4
							end := strings.Index(pgMarTag[start:], `"`)
							if end != -1 {
								if value := dp.parseFloatFromTwips(pgMarTag[start:start+end]); value > 0 {
									*ptr = value
								}
							}
						}
					}
				}
			}
		}
	}

	return layout
}

// Convert twentieths of a point to points
func (dp *DocxProcessor) parseFloatFromTwips(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Remove any non-digit characters except decimal point
	var numStr strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			numStr.WriteRune(r)
		}
	}

	// Parse the number
	if numStr.Len() == 0 {
		return 0
	}

	// Simple parsing without external dependencies
	str := numStr.String()
	var result float64
	var decimal float64 = 1
	var afterDecimal bool

	for _, r := range str {
		if r == '.' {
			afterDecimal = true
			continue
		}

		digit := float64(r - '0')
		if afterDecimal {
			decimal *= 10
			result += digit / decimal
		} else {
			result = result*10 + digit
		}
	}

	// Convert from twentieths of a point to points
	return result / 20.0
}

// Calculate coordinates based on XML position and document structure
func (dp *DocxProcessor) calculateCoordinates(xmlContent string, xmlPos int, layout DocumentLayout) (float64, float64, int, string) {
	// Find the paragraph containing this position
	paragraphInfo := dp.findContainingParagraph(xmlContent, xmlPos)

	// Calculate Y position based on paragraph line number
	y := layout.TopMargin + (float64(paragraphInfo.LineNumber-1) * layout.LineHeight)

	// Calculate X position (simplified - assumes left-aligned text)
	x := layout.LeftMargin + paragraphInfo.X

	// Estimate page number based on Y position
	pageNumber := int(y/layout.PageHeight) + 1
	if pageNumber < 1 {
		pageNumber = 1
	}

	// Adjust Y for page offset
	y = y - (float64(pageNumber-1) * layout.PageHeight)

	return x, y, pageNumber, paragraphInfo.ParagraphId
}

// Find paragraph information containing the XML position
func (dp *DocxProcessor) findContainingParagraph(content string, xmlPos int) ParagraphInfo {
	info := ParagraphInfo{
		XMLPosition: xmlPos,
		LineNumber:  1,
		X:           0,
		Y:           0,
		Width:       0,
		IsInTable:   false,
		TableRow:    0,
		TableCol:    0,
		ParagraphId: "p1",
	}

	// Count paragraphs before this position
	paragraphCount := 0
	searchPos := 0

	for searchPos < xmlPos && searchPos < len(content) {
		// Look for paragraph tags
		if pStart := strings.Index(content[searchPos:], "<w:p "); pStart != -1 {
			pStart += searchPos
			if pStart < xmlPos {
				paragraphCount++

				// Look for paragraph ID
				pEnd := strings.Index(content[pStart:], ">")
				if pEnd != -1 {
					// Extract paragraph properties if available
					info.ParagraphId = fmt.Sprintf("p%d", paragraphCount)
				}

				searchPos = pStart + 1
			} else {
				break
			}
		} else {
			break
		}
	}

	info.LineNumber = paragraphCount
	if info.LineNumber < 1 {
		info.LineNumber = 1
	}

	// Check if inside table
	tableStart := strings.LastIndex(content[:xmlPos], "<w:tbl>")
	tableEnd := strings.LastIndex(content[:xmlPos], "</w:tbl>")
	if tableStart > tableEnd {
		info.IsInTable = true
		// Count table rows and cells (simplified)
		rowCount := strings.Count(content[tableStart:xmlPos], "<w:tr>")
		cellCount := strings.Count(content[tableStart:xmlPos], "<w:tc>")
		info.TableRow = rowCount
		info.TableCol = cellCount % 10 // Rough estimate
	}

	return info
}

// Estimate text width based on character count and font size
func (dp *DocxProcessor) estimateTextWidth(text string, fontSize float64) float64 {
	if fontSize <= 0 {
		fontSize = 12 // Default font size
	}

	// Rough estimate: average character width is about 0.6 * font size
	charWidth := fontSize * 0.6
	return float64(len(text)) * charWidth
}

// Convert coordinates to SVG coordinate system
// SVG origin (0,0) is typically at top-left, same as our document coordinates
func (dp *DocxProcessor) ConvertToSVGCoordinates(x, y, pageWidth, pageHeight float64) (float64, float64) {
	// For SVG mapping, coordinates are already in the correct format
	// Points can be converted to pixels by multiplying by DPI/72
	// For web SVG, you might want to scale to viewport size

	svgX := x
	svgY := y

	return svgX, svgY
}

// Convert points to pixels (standard web DPI is 96)
func (dp *DocxProcessor) PointsToPixels(points float64, dpi float64) float64 {
	if dpi <= 0 {
		dpi = 96 // Standard web DPI
	}
	return points * (dpi / 72.0)
}

// Helper function to get SVG-ready coordinates
func (dp *DocxProcessor) GetSVGCoordinates(position PlaceholderPosition, targetDPI float64) (float64, float64, float64, float64) {
	if targetDPI <= 0 {
		targetDPI = 96 // Default to web DPI
	}

	svgX := dp.PointsToPixels(position.X, targetDPI)
	svgY := dp.PointsToPixels(position.Y, targetDPI)
	svgWidth := dp.PointsToPixels(position.Width, targetDPI)
	svgHeight := dp.PointsToPixels(position.Height, targetDPI)

	return svgX, svgY, svgWidth, svgHeight
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

