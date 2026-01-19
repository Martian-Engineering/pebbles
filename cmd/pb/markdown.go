package main

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	codeSpanPattern = regexp.MustCompile("`[^`]+`")
	boldPattern     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	boldAltPattern  = regexp.MustCompile(`__([^_]+)__`)
)

// codeSpan tracks inline code segments that are masked during styling.
type codeSpan struct {
	token string
	value string
}

// renderMarkdown applies lightweight ANSI styling for common markdown patterns.
func renderMarkdown(text string) string {
	if !colorEnabled {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = highlightMarkdownLine(line)
	}
	return strings.Join(lines, "\n")
}

// highlightMarkdownLine applies markdown styling to a single line of text.
func highlightMarkdownLine(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}
	if highlighted, ok := highlightHeading(line); ok {
		return highlighted
	}
	masked, spans := maskCodeSpans(line)
	masked = highlightBullet(masked)
	masked = highlightBold(masked)
	return restoreCodeSpans(masked, spans)
}

// maskCodeSpans replaces inline code spans with tokens and returns the mapping.
func maskCodeSpans(line string) (string, []codeSpan) {
	matches := codeSpanPattern.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return line, nil
	}
	// Replace spans with stable tokens so other formatters don't touch them.
	spans := make([]codeSpan, 0, len(matches))
	var builder strings.Builder
	last := 0
	for i, match := range matches {
		token := fmt.Sprintf("{CODE_%d}", i)
		// Append the untouched text before the code span.
		builder.WriteString(line[last:match[0]])
		builder.WriteString(token)
		spans = append(spans, codeSpan{
			token: token,
			value: line[match[0]:match[1]],
		})
		last = match[1]
	}
	// Finish the line after the last code span.
	builder.WriteString(line[last:])
	return builder.String(), spans
}

// restoreCodeSpans reinserts colored code spans into a masked line.
func restoreCodeSpans(line string, spans []codeSpan) string {
	if len(spans) == 0 {
		return line
	}
	restored := line
	for _, span := range spans {
		restored = strings.ReplaceAll(restored, span.token, colorize(span.value, ansiCyan))
	}
	return restored
}

// highlightHeading applies accent styling to markdown headings.
func highlightHeading(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "#") {
		return line, false
	}
	index := strings.Index(trimmed, " ")
	if index <= 0 {
		return line, false
	}
	if strings.Trim(trimmed[:index], "#") != "" {
		return line, false
	}
	return colorize(line, ansiBold+ansiBrightBlue), true
}

// highlightBullet accentuates markdown list bullets.
func highlightBullet(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 2 {
		return line
	}
	bullet := trimmed[0]
	if (bullet != '-' && bullet != '*' && bullet != '+') || trimmed[1] != ' ' {
		return line
	}
	indent := line[:len(line)-len(trimmed)]
	return indent + colorize(string(bullet), ansiBrightBlue) + trimmed[1:]
}

// highlightBold applies emphasis styling to markdown bold text.
func highlightBold(line string) string {
	line = boldPattern.ReplaceAllStringFunc(line, func(match string) string {
		return colorize(match[2:len(match)-2], ansiBold)
	})
	return boldAltPattern.ReplaceAllStringFunc(line, func(match string) string {
		return colorize(match[2:len(match)-2], ansiBold)
	})
}
