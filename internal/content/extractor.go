package content

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"github.com/xuri/excelize/v2"
)

// ExtractText attempts to pull raw text from supported file formats.
// Returns an error if the format is unsupported or parsing fails.
func ExtractText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".txt", ".csv", ".log", ".md", ".json", ".xml", ".html", ".js", ".ts", ".go":
		// Plain text formats
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(content), nil

	case ".pdf":
		return extractPDF(path)

	case ".docx":
		return extractDOCX(path)

	case ".xlsx":
		return extractXLSX(path)
	
	case ".jpg", ".jpeg", ".png":
		return extractImageMetadata(path)

	default:
		return "", fmt.Errorf("unsupported format: %s", ext)
	}
}

func extractPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	buf.ReadFrom(b)
	return buf.String(), nil
}

func extractDOCX(path string) (string, error) {
	r, err := docx.ReadDocxFile(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	content := r.Editable()
	return content.GetContent(), nil
}

func extractXLSX(path string) (string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for _, row := range rows {
			for _, colCell := range row {
				sb.WriteString(colCell)
				sb.WriteString(" ")
			}
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

func extractImageMetadata(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Decode image config to verify it's valid
	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		return "", err
	}

	// Build metadata string
	filename := filepath.Base(path)
	var metadata strings.Builder
	
	metadata.WriteString(fmt.Sprintf("Image Analysis: %s\n", filename))
	metadata.WriteString(fmt.Sprintf("Format: %s (%dx%d pixels)\n", format, cfg.Width, cfg.Height))
	metadata.WriteString(fmt.Sprintf("File Path: %s\n", path))
	
	// Analyze filename for PHI indicators
	lowerName := strings.ToLower(filename)
	sensitiveTerms := []string{"patient", "medical", "xray", "mri", "ct-scan", "diagnosis", "hipaa"}
	foundTerms := []string{}
	
	for _, term := range sensitiveTerms {
		if strings.Contains(lowerName, term) {
			foundTerms = append(foundTerms, term)
		}
	}
	
	if len(foundTerms) > 0 {
		metadata.WriteString(fmt.Sprintf("\nPotential PHI Indicators in Filename: %s\n", strings.Join(foundTerms, ", ")))
	}
	
	return metadata.String(), nil
}

