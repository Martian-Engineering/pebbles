package main

import (
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

const defaultMarkdownWidth = 80

// renderMarkdown renders markdown to ANSI when color output is enabled.
func renderMarkdown(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	if !colorEnabled {
		return text
	}
	rendered, err := renderMarkdownWithGlamour(text)
	if err != nil {
		return text
	}
	rendered = strings.TrimRight(rendered, "\n")
	return strings.TrimLeft(rendered, "\n")
}

// renderMarkdownWithGlamour renders markdown using glamour with terminal-aware options.
func renderMarkdownWithGlamour(text string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(markdownStyle()),
		glamour.WithWordWrap(markdownWordWrap()),
	)
	if err != nil {
		return "", err
	}
	return renderer.Render(text)
}

// markdownWordWrap returns the terminal width or a safe default.
func markdownWordWrap() int {
	if !isTTY(os.Stdout) {
		return defaultMarkdownWidth
	}
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return defaultMarkdownWidth
	}
	return width
}

// markdownStyle selects a built-in glamour style based on the terminal background.
func markdownStyle() string {
	if termenv.HasDarkBackground() {
		return styles.DarkStyle
	}
	return styles.LightStyle
}
