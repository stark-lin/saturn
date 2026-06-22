// This file parses and validates the current Markdown source for a Note.
package notes

import (
	"errors"
	"strings"
)

var ErrInvalidMarkdown = errors.New("invalid note markdown")

type ParsedMarkdown struct {
	Title string
	Tags  []string
}

func ParseMarkdown(markdown string) (ParsedMarkdown, error) {
	lines := strings.Split(markdown, "\n")
	if len(lines) < 2 {
		return ParsedMarkdown{}, ErrInvalidMarkdown
	}

	title := strings.TrimSpace(strings.TrimSuffix(lines[0], "\r"))

	tagLine := strings.TrimSuffix(lines[1], "\r")
	seen := make(map[string]struct{})
	tags := make([]string, 0)
	for _, rawTag := range strings.Split(tagLine, ",") {
		name := strings.TrimSpace(rawTag)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}

	return ParsedMarkdown{Title: title, Tags: tags}, nil
}
