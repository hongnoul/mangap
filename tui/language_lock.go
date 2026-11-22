package tui

import "strings"

// languageLocked reports whether the current source only serves English, so
// the language preference cannot apply. Mangapill is English only.
func (b *statefulBubble) languageLocked() bool {
	if b.sourceIdx >= len(b.sourceChoices) {
		return false
	}
	return strings.EqualFold(b.sourceChoices[b.sourceIdx], "mangapill")
}
