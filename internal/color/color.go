// Package color provides ANSI color utilities and JSON syntax highlighting
// for terminal error output.
package color

import (
	"strconv"
	"strings"
	"unicode"
)

const (
	reset   = "\033[0m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
)

var (
	Red    = red
	Blue   = blue
	Cyan   = cyan
	Gray   = gray
	Green  = green
	Yellow = yellow
	Reset  = reset
)

func RedText(s string) string { return red + s + reset }

// ColorizeJSON applies syntax highlighting to a JSON string.
// Keys are blue, string values are green, numbers are yellow,
// booleans/null are magenta, structural characters are gray.
func ColorizeJSON(raw string) string {
	if raw == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(raw) + len(raw)/4) // ~25% overhead for escape codes

	runes := []rune(raw)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		switch {
		case ch == '"':
			i = colorizeString(&b, runes, i)

		case ch == '-' || (ch >= '0' && ch <= '9'):
			i = colorizeNumber(&b, runes, i)

		case ch == 't' || ch == 'f' || ch == 'n':
			i = colorizeLiteral(&b, runes, i)

		case ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ',':
			b.WriteString(gray)
			b.WriteRune(ch)
			b.WriteString(reset)
			i++

		case ch == ':':
			b.WriteString(gray)
			b.WriteRune(ch)
			b.WriteString(reset)
			i++

		default:
			b.WriteRune(ch)
			i++
		}
	}
	return b.String()
}

func colorizeString(b *strings.Builder, runes []rune, start int) int {
	end := start + 1
	escaped := false
	for end < len(runes) {
		ch := runes[end]
		if escaped {
			escaped = false
			end++
			continue
		}
		if ch == '\\' {
			escaped = true
			end++
			continue
		}
		if ch == '"' {
			end++ // include closing quote
			break
		}
		end++
	}

	// Determine if this is a key (followed by ':') or a value.
	isKey := false
	j := end
	for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r') {
		j++
	}
	if j < len(runes) && runes[j] == ':' {
		isKey = true
	}

	content := string(runes[start:end])
	if isKey {
		b.WriteString(blue)
		b.WriteString(content)
		b.WriteString(reset)
	} else {
		b.WriteString(green)
		b.WriteString(content)
		b.WriteString(reset)
	}
	return end
}

func colorizeNumber(b *strings.Builder, runes []rune, start int) int {
	end := start
	for end < len(runes) && (runes[end] == '-' || runes[end] == '.' ||
		runes[end] == 'e' || runes[end] == 'E' || runes[end] == '+' ||
		(runes[end] >= '0' && runes[end] <= '9')) {
		end++
	}
	// Validate it's actually a number (not just a stray '-').
	content := string(runes[start:end])
	if _, err := strconv.ParseFloat(content, 64); err == nil {
		b.WriteString(yellow)
		b.WriteString(content)
		b.WriteString(reset)
	} else {
		b.WriteString(content)
	}
	return end
}

func colorizeLiteral(b *strings.Builder, runes []rune, start int) int {
	// Try to match true, false, null.
	remaining := string(runes[start:])
	for _, lit := range []string{"true", "false", "null"} {
		if strings.HasPrefix(remaining, lit) {
			// Check that the next char is a separator to avoid partial matches.
			end := start + len(lit)
			if end == len(runes) || !isIdentChar(runes[end]) {
				b.WriteString(magenta)
				b.WriteString(lit)
				b.WriteString(reset)
				return end
			}
		}
	}
	b.WriteRune(runes[start])
	return start + 1
}

func isIdentChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

// ColorizeKeyValueBlock colorizes a block of key: value lines.
// Keys are cyan, colons and spacing are gray, string values are green,
// numeric values are yellow.
func ColorizeKeyValueBlock(text string) string {
	var b strings.Builder
	b.Grow(len(text) + len(text)/4)

	for _, line := range strings.SplitAfter(text, "\n") {
		if line == "" {
			continue
		}
		idx := strings.Index(line, ": ")
		if idx < 0 {
			// "Key:" without value (header line before JSON)
			idx = strings.Index(line, ":")
			if idx >= 0 {
				b.WriteString(cyan)
				b.WriteString(line[:idx])
				b.WriteString(reset)
				b.WriteString(gray)
				b.WriteString(":")
				b.WriteString(reset)
				b.WriteString(line[idx+1:])
			} else {
				b.WriteString(line)
			}
			continue
		}
		keyWithIndent := line[:idx]
		colonSpace := ": "
		value := line[idx+2:]

		b.WriteString(cyan)
		b.WriteString(keyWithIndent)
		b.WriteString(reset)
		b.WriteString(gray)
		b.WriteString(colonSpace)
		b.WriteString(reset)

		trimmed := strings.TrimSpace(value)
		if isNumeric(trimmed) {
			b.WriteString(yellow)
		} else {
			b.WriteString(green)
		}
		// Value may include trailing newline (from SplitAfter)
		b.WriteString(value)
		b.WriteString(reset)
	}
	return b.String()
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// Indent wraps a string with indentation prefix on each line.
func Indent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
