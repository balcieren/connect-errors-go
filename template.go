package connecterrors

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var templateRegex = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

// templatePart represents a segment of a parsed template.
// Each part has a literal prefix and an optional placeholder field name.
type templatePart struct {
	literal string // literal text before the placeholder
	field   string // placeholder field name; empty for trailing literal
}

// partsCache caches parsed template parts keyed by template string.
// sync.Map is ideal here: written once per unique template, read many times.
var partsCache sync.Map

// parseTemplateParts splits a template string into literal+placeholder segments.
func parseTemplateParts(tpl string) []templatePart {
	matches := templateRegex.FindAllStringSubmatchIndex(tpl, -1)
	if len(matches) == 0 {
		return []templatePart{{literal: tpl}}
	}

	parts := make([]templatePart, 0, len(matches)+1)
	last := 0
	for _, m := range matches {
		parts = append(parts, templatePart{
			literal: tpl[last:m[0]],
			field:   tpl[m[2]:m[3]],
		})
		last = m[1]
	}
	if last < len(tpl) {
		parts = append(parts, templatePart{literal: tpl[last:]})
	}
	return parts
}

// cachedParts returns pre-parsed template parts, computing and caching on first call.
func cachedParts(tpl string) []templatePart {
	if v, ok := partsCache.Load(tpl); ok {
		return v.([]templatePart)
	}
	parts := parseTemplateParts(tpl)
	partsCache.Store(tpl, parts)
	return parts
}

// TemplateFields extracts all unique placeholder field names from a template string.
// Fields are returned in sorted order for deterministic output.
func TemplateFields(template string) []string {
	matches := templateRegex.FindAllStringSubmatch(template, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(matches))
	fields := make([]string, 0, len(matches))

	for _, match := range matches {
		field := match[1]
		if !seen[field] {
			seen[field] = true
			fields = append(fields, field)
		}
	}

	sort.Strings(fields)
	return fields
}

// MissingFieldError is returned when a template requires fields
// that are not provided in the data map.
type MissingFieldError struct {
	Template string
	Missing  []string
}

// Error implements the error interface for MissingFieldError.
func (e *MissingFieldError) Error() string {
	return fmt.Sprintf("template %q missing fields: %s", e.Template, strings.Join(e.Missing, ", "))
}

// ValidateTemplate checks whether all required template fields are present in the data map.
// Returns a *MissingFieldError if any fields are missing, or nil if all fields are provided.
func ValidateTemplate(template string, data M) error {
	fields := TemplateFields(template)
	if len(fields) == 0 {
		return nil
	}

	var missing []string
	for _, field := range fields {
		if _, ok := data[field]; !ok {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		return &MissingFieldError{
			Template: template,
			Missing:  missing,
		}
	}

	return nil
}

// FormatTemplate replaces all placeholders in the template with corresponding
// values from the data map. Unmatched placeholders are left unchanged.
//
// Uses cached pre-parsed template parts to avoid regex execution on repeated calls.
func FormatTemplate(template string, data M) string {
	if len(data) == 0 {
		return template
	}

	parts := cachedParts(template)

	var b strings.Builder
	b.Grow(len(template))
	for _, p := range parts {
		b.WriteString(p.literal)
		if p.field != "" {
			if val, ok := data[p.field]; ok {
				b.WriteString(val)
			} else {
				b.WriteString("{{")
				b.WriteString(p.field)
				b.WriteString("}}")
			}
		}
	}
	return b.String()
}
