package scml

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

const (
	scmlContractOpenComment   = "<!-- <contract> -->"
	scmlContractCloseComment  = "<!-- </contract> -->"
	scmlConstantsOpenComment  = "<!-- <constants> -->"
	scmlConstantsCloseComment = "<!-- </constants> -->"
	scmlSectionCloseComment   = "<!-- </section> -->"
	scmlPreOpen               = "<pre>"
	scmlPreClose              = "</pre>"
)

func normalizeSCMLDocument(content string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var out bytes.Buffer

	inContract := false
	inConstants := false
	inPre := false
	sectionDepth := 0
	sawConstantsClose := false
	sawContractClose := false

	for lineNo := 1; scanner.Scan(); lineNo++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)

		if line == "" {
			continue
		}

		switch {
		case line == scmlContractOpenComment:
			if inContract {
				return "", fmt.Errorf("line %d: duplicate <!-- <contract> --> marker", lineNo)
			}
			if sawContractClose {
				return "", fmt.Errorf("line %d: duplicate <!-- <contract> --> marker after contract close", lineNo)
			}
			inContract = true
			out.WriteString("<contract>")
			continue
		case line == scmlContractCloseComment:
			if !inContract {
				return "", fmt.Errorf("line %d: unexpected <!-- </contract> --> marker", lineNo)
			}
			if inConstants || inPre {
				return "", fmt.Errorf("line %d: contract closed before <constants> block completed", lineNo)
			}
			if sectionDepth > 0 {
				return "", fmt.Errorf("line %d: contract closed before all <section> blocks were closed", lineNo)
			}
			if !sawConstantsClose {
				return "", fmt.Errorf("line %d: contract must contain a complete <constants> block", lineNo)
			}
			out.WriteString("</contract>")
			inContract = false
			sawContractClose = true
			continue
		case !inContract:
			if sawContractClose {
				if line != "" {
					return "", fmt.Errorf("line %d: content after <!-- </contract> --> is not allowed", lineNo)
				}
				continue
			}
			if isSCMLCommentCandidate(line) {
				return "", fmt.Errorf("line %d: invalid SCML comment marker %q", lineNo, line)
			}
			if isRawXMLLikeLine(line) {
				return "", fmt.Errorf("line %d: raw XML-like syntax must be wrapped in HTML comments", lineNo)
			}
			continue
		case line == scmlConstantsOpenComment:
			if inConstants || inPre || sectionDepth > 0 || sawConstantsClose {
				return "", fmt.Errorf("line %d: duplicate or misplaced <constants> block", lineNo)
			}
			inConstants = true
			out.WriteString("<constants>")
			continue
		case line == scmlConstantsCloseComment:
			if !inConstants || inPre {
				return "", fmt.Errorf("line %d: unexpected <!-- </constants> --> marker", lineNo)
			}
			out.WriteString("</constants>")
			inConstants = false
			sawConstantsClose = true
			continue
		case line == scmlPreOpen:
			if !inConstants || inPre {
				return "", fmt.Errorf("line %d: unexpected <pre> marker", lineNo)
			}
			inPre = true
			out.WriteString("<pre>")
			continue
		case line == scmlPreClose:
			if !inPre {
				return "", fmt.Errorf("line %d: unexpected </pre> marker", lineNo)
			}
			out.WriteString("</pre>")
			inPre = false
			continue
		}

		if inConstants {
			if inPre {
				if isSCMLCommentCandidate(line) {
					return "", fmt.Errorf("line %d: invalid SCML comment marker inside <pre>: %q", lineNo, line)
				}
				if isRawXMLLikeLine(line) {
					return "", fmt.Errorf("line %d: raw XML-like syntax must be wrapped in HTML comments", lineNo)
				}
				if err := xml.EscapeText(&out, []byte(rawLine)); err != nil {
					return "", fmt.Errorf("line %d: escape constants text: %w", lineNo, err)
				}
				out.WriteByte('\n')
				continue
			}
			if isSCMLCommentCandidate(line) {
				return "", fmt.Errorf("line %d: invalid SCML comment marker inside <constants>: %q", lineNo, line)
			}
			return "", fmt.Errorf("line %d: unexpected content inside <constants>: %q", lineNo, line)
		}

		if line == scmlSectionCloseComment {
			if sectionDepth == 0 {
				return "", fmt.Errorf("line %d: unexpected <!-- </section> --> marker", lineNo)
			}
			out.WriteString("</section>")
			sectionDepth--
			continue
		}

		if strings.HasPrefix(line, "<!-- <section ") && strings.HasSuffix(line, " -->") {
			if !sawConstantsClose {
				return "", fmt.Errorf("line %d: <section> blocks must appear after a complete <constants> block", lineNo)
			}
			start, err := parseSectionMarker(line)
			if err != nil {
				return "", fmt.Errorf("line %d: %w", lineNo, err)
			}
			out.WriteString("<section")
			for _, attr := range start.Attr {
				out.WriteByte(' ')
				out.WriteString(attr.Name.Local)
				out.WriteString(`="`)
				out.WriteString(escapeXMLAttrValue(attr.Value))
				out.WriteByte('"')
			}
			out.WriteString(">")
			sectionDepth++
			continue
		}

		if sectionDepth > 0 {
			if strings.HasPrefix(line, "- ") {
				item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
				if item == "" {
					return "", fmt.Errorf("line %d: empty section item is not allowed", lineNo)
				}
				out.WriteString("<item>")
				if err := xml.EscapeText(&out, []byte(item)); err != nil {
					return "", fmt.Errorf("line %d: escape section item: %w", lineNo, err)
				}
				out.WriteString("</item>")
				continue
			}
			if strings.HasPrefix(line, "## ") {
				continue
			}
			if strings.HasPrefix(line, ">") {
				continue
			}
			if isSCMLCommentCandidate(line) {
				return "", fmt.Errorf("line %d: invalid SCML comment marker inside <section>: %q", lineNo, line)
			}
			if isRawXMLLikeLine(line) {
				return "", fmt.Errorf("line %d: raw XML-like syntax must be wrapped in HTML comments", lineNo)
			}
			return "", fmt.Errorf("line %d: unexpected content inside <section>: %q", lineNo, line)
		}

		if isSCMLCommentCandidate(line) {
			return "", fmt.Errorf("line %d: invalid SCML comment marker %q", lineNo, line)
		}
		if isRawXMLLikeLine(line) {
			return "", fmt.Errorf("line %d: raw XML-like syntax must be wrapped in HTML comments", lineNo)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan SCML content: %w", err)
	}
	if inContract {
		return "", fmt.Errorf("SCML contract root <!-- <contract> --> not closed")
	}
	if inConstants || inPre {
		return "", fmt.Errorf("SCML contract ended inside <constants> block")
	}
	if sectionDepth > 0 {
		return "", fmt.Errorf("SCML contract ended with %d unclosed <section> block(s)", sectionDepth)
	}

	return out.String(), nil
}

func isSCMLCommentCandidate(line string) bool {
	return strings.HasPrefix(line, "<!-- <") || strings.HasPrefix(line, "<!-- </")
}

func parseSectionMarker(line string) (xml.StartElement, error) {
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "<!--"), "-->"))
	inner = strings.TrimSpace(inner)
	if !strings.HasPrefix(inner, "<section") || !strings.HasSuffix(inner, ">") {
		return xml.StartElement{}, fmt.Errorf("invalid section marker %q", line)
	}

	snippet := inner + "</section>"
	decoder := xml.NewDecoder(strings.NewReader(snippet))

	tok, err := decoder.Token()
	if err != nil {
		return xml.StartElement{}, fmt.Errorf("parse section marker: %w", err)
	}
	start, ok := tok.(xml.StartElement)
	if !ok {
		return xml.StartElement{}, fmt.Errorf("parse section marker: expected start element")
	}
	return start, nil
}

func escapeXMLAttrValue(value string) string {
	var builder strings.Builder
	if err := xml.EscapeText(&builder, []byte(value)); err != nil {
		return value
	}
	return strings.ReplaceAll(builder.String(), `"`, "&quot;")
}
