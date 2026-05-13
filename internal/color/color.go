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
	gray    = "\033[90m"
)

var (
	Red   = red
	Reset = reset
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

// Indent wraps a string with indentation prefix on each line.
func Indent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
