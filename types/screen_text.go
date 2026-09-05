package types

import (
	"fmt"
	"strings"
)

// FormatText renders a ref-annotated tree as indented lines, one element per
// line, e.g. `  @e7 [Button] "Add" #add-button (disabled)`. It is far more
// compact than JSON, which matters when the tree is fed to a model.
// Call AttachRefs first, otherwise refs render empty.
func FormatText(elements []ScreenElement) string {
	var builder strings.Builder
	writeElementLines(&builder, elements, 0)
	return builder.String()
}

func writeElementLines(builder *strings.Builder, elements []ScreenElement, depth int) {
	for _, element := range elements {
		builder.WriteString(strings.Repeat("  ", depth))
		builder.WriteString(formatElementLine(element))
		builder.WriteString("\n")
		writeElementLines(builder, element.Children, depth+1)
	}
}

func formatElementLine(element ScreenElement) string {
	parts := []string{fmt.Sprintf("@%s [%s]", element.Ref, element.Type)}

	if description := elementDescription(element); description != "" {
		parts = append(parts, fmt.Sprintf("%q", description))
	}

	// The identifier is what a caller would write a selector against, so it stays
	// on the line even when a label is already shown.
	if element.Identifier != nil && *element.Identifier != "" {
		parts = append(parts, "#"+*element.Identifier)
	}

	if states := elementStates(element); states != "" {
		parts = append(parts, states)
	}

	return strings.Join(parts, " ")
}

// elementDescription is the first non-empty human-readable field, in the order
// an agent would care about.
func elementDescription(element ScreenElement) string {
	for _, candidate := range []*string{element.Text, element.Label, element.Name, element.Value, element.Placeholder} {
		if candidate != nil && *candidate != "" {
			return *candidate
		}
	}
	return ""
}

func elementStates(element ScreenElement) string {
	var states []string
	if element.Enabled != nil && !*element.Enabled {
		states = append(states, "disabled")
	}
	if element.Checked != nil && *element.Checked {
		states = append(states, "checked")
	}
	if element.Selected != nil && *element.Selected {
		states = append(states, "selected")
	}
	if element.Focused != nil && *element.Focused {
		states = append(states, "focused")
	}
	if len(states) == 0 {
		return ""
	}
	return "(" + strings.Join(states, ", ") + ")"
}
