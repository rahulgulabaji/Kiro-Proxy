package reqconv

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/d-kuro/kirocc/internal/anthropic"
)

// ExtractDocumentContent extracts human-readable text from an Anthropic "document" block.
func ExtractDocumentContent(b anthropic.ContentBlock) string {
	title := b.Title
	if title == "" {
		title = b.Name
	}
	if title == "" {
		title = "Attached Document"
	}

	if b.Source == nil {
		return fmt.Sprintf("[Document: %s]", title)
	}

	// 1. Text source
	if b.Source.Type == "text" || b.Source.MediaType == "text/plain" || b.Source.MediaType == "text/markdown" {
		return fmt.Sprintf("\n\n--- Document: %s ---\n%s\n--- End of %s ---\n", title, b.Source.Data, title)
	}

	// 2. Base64 source
	if b.Source.Type == "base64" && b.Source.Data != "" {
		rawBytes, err := base64.StdEncoding.DecodeString(b.Source.Data)
		if err != nil {
			// Try URL-safe or raw decoding
			rawBytes, err = base64.RawStdEncoding.DecodeString(b.Source.Data)
		}
		if err != nil {
			return fmt.Sprintf("[Document: %s (base64 decode error)]", title)
		}

		// Check if DOCX (ZIP format starting with PK)
		if bytes.HasPrefix(rawBytes, []byte("PK\x03\x04")) || strings.Contains(b.Source.MediaType, "wordprocessingml") {
			if text, err := extractTextFromDocx(rawBytes); err == nil && text != "" {
				return fmt.Sprintf("\n\n--- Document (DOCX): %s ---\n%s\n--- End of %s ---\n", title, text, title)
			}
		}

		// Check if PDF
		if bytes.HasPrefix(rawBytes, []byte("%PDF-")) || b.Source.MediaType == "application/pdf" {
			if text := extractTextFromPDF(rawBytes); text != "" {
				return fmt.Sprintf("\n\n--- Document (PDF): %s ---\n%s\n--- End of %s ---\n", title, text, title)
			}
		}

		// Check if plain UTF-8 text (e.g. csv, json, code, markdown)
		if utf8.Valid(rawBytes) {
			cleanText := strings.TrimSpace(string(rawBytes))
			if cleanText != "" {
				return fmt.Sprintf("\n\n--- Document: %s ---\n%s\n--- End of %s ---\n", title, cleanText, title)
			}
		}
	}

	return fmt.Sprintf("[Document: %s]", title)
}

// extractTextFromDocx unzips a DOCX archive and extracts XML text from word/document.xml.
func extractTextFromDocx(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	for _, f := range reader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			xmlData, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return "", err
			}

			return parseWordXML(xmlData), nil
		}
	}

	return "", fmt.Errorf("word/document.xml not found")
}

// parseWordXML parses DOCX XML tokens into clean paragraph text.
func parseWordXML(xmlData []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var sb strings.Builder

	for {
		t, err := decoder.Token()
		if err != nil {
			break
		}
		switch elem := t.(type) {
		case xml.StartElement:
			if elem.Name.Local == "p" && sb.Len() > 0 {
				sb.WriteString("\n")
			}
		case xml.CharData:
			sb.Write(elem)
		}
	}

	return strings.TrimSpace(sb.String())
}

var (
	pdfStreamRegex = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	pdfTjRegex     = regexp.MustCompile(`\((.*?)\)\s*Tj`)
	pdfTJRegex     = regexp.MustCompile(`\[(.*?)\]\s*TJ`)
	pdfTextInArray = regexp.MustCompile(`\((.*?)\)`)
)

// extractTextFromPDF scans uncompressed and zlib-compressed PDF streams for text.
func extractTextFromPDF(data []byte) string {
	var sb strings.Builder

	matches := pdfStreamRegex.FindAllSubmatch(data, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		streamData := m[1]

		// Try decompressing with zlib/FlateDecode
		zr, err := zlib.NewReader(bytes.NewReader(streamData))
		if err == nil {
			decompressed, err := io.ReadAll(zr)
			_ = zr.Close()
			if err == nil {
				streamData = decompressed
			}
		}

		// Extract (text) Tj
		for _, tjMatch := range pdfTjRegex.FindAllSubmatch(streamData, -1) {
			if len(tjMatch) >= 2 {
				txt := cleanPDFString(string(tjMatch[1]))
				if txt != "" {
					sb.WriteString(txt)
					sb.WriteString(" ")
				}
			}
		}

		// Extract [(text) -10 (more)] TJ
		for _, tjMatch := range pdfTJRegex.FindAllSubmatch(streamData, -1) {
			if len(tjMatch) >= 2 {
				for _, inner := range pdfTextInArray.FindAllSubmatch(tjMatch[1], -1) {
					if len(inner) >= 2 {
						txt := cleanPDFString(string(inner[1]))
						if txt != "" {
							sb.WriteString(txt)
						}
					}
				}
				sb.WriteString(" ")
			}
		}
	}

	result := strings.TrimSpace(sb.String())
	if len(result) > 50 {
		return result
	}

	// Fallback: extract printable ASCII strings
	var fallback strings.Builder
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			fallback.WriteByte(b)
		} else if fallback.Len() > 0 && fallback.String()[fallback.Len()-1] != ' ' {
			fallback.WriteByte(' ')
		}
	}
	return strings.TrimSpace(fallback.String())
}

func cleanPDFString(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\r`, "\r")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\(`, "(")
	s = strings.ReplaceAll(s, `\)`, ")")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

