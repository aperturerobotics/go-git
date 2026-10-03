package diff

import (
	"cmp"
	"slices"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// textEdit replaces a byte range in the common base with one side's text.
type textEdit struct {
	// start is the first byte replaced in the base.
	start int
	// end is the first byte after the replaced range.
	end int
	// text replaces the range, including insertions at an empty range.
	text string
	// ours identifies which side supplied the edit.
	ours bool
}

// Merge combines line edits from ours and theirs relative to base. Independent
// edits survive; overlapping edits use ours when preferOurs is true, or theirs
// otherwise. Identical changes are applied once.
func Merge(base, ours, theirs string, preferOurs bool) string {
	// Order both sides' edits by their position in the common base.
	edits := append(textEdits(base, ours, true), textEdits(base, theirs, false)...)
	slices.SortStableFunc(edits, func(a, b textEdit) int {
		return cmp.Compare(a.start, b.start)
	})

	// Apply connected edit ranges while retaining untouched base text.
	var result strings.Builder
	position := 0
	for i := 0; i < len(edits); {
		// Collect overlapping edits into one range, including shared insertions.
		start, end := edits[i].start, edits[i].end
		j := i + 1
		for j < len(edits) && edits[j].start <= end {
			end = max(end, edits[j].end)
			j++
		}
		result.WriteString(base[position:start])

		// Render each side over the same base range before selecting a conflict.
		ourText, ourChanged := applyTextEdits(base, start, end, edits[i:j], true)
		theirText, theirChanged := applyTextEdits(base, start, end, edits[i:j], false)
		switch {
		case !theirChanged || ourChanged && (ourText == theirText || preferOurs):
			result.WriteString(ourText)
		default:
			result.WriteString(theirText)
		}
		position = end
		i = j
	}

	// Append the unchanged suffix after the last edit.
	result.WriteString(base[position:])
	return result.String()
}

// textEdits coalesces each line-diff replacement into one base-relative edit.
func textEdits(base, text string, ours bool) []textEdit {
	// Track base offsets independently of inserted text.
	var edits []textEdit
	position := 0
	var edit *textEdit
	for _, change := range Do(base, text) {
		// Equal text separates replacements and advances only the base offset.
		if change.Type == diffmatchpatch.DiffEqual {
			position += len(change.Text)
			edit = nil
			continue
		}

		// Keep adjacent insertions and deletions in the same replacement.
		if edit == nil {
			edits = append(edits, textEdit{start: position, end: position, ours: ours})
			edit = &edits[len(edits)-1]
		}
		if change.Type == diffmatchpatch.DiffDelete {
			position += len(change.Text)
			edit.end = position
			continue
		}
		edit.text += change.Text
	}

	// Return edits in the base order supplied by the line diff.
	return edits
}

// applyTextEdits renders one side of a connected range and reports whether that
// side changed any part of the range.
func applyTextEdits(base string, start, end int, edits []textEdit, ours bool) (string, bool) {
	// Apply the selected side's edits, retaining base gaps inside the range.
	var result strings.Builder
	position := start
	changed := false
	for _, edit := range edits {
		if edit.ours != ours {
			continue
		}
		result.WriteString(base[position:edit.start])
		result.WriteString(edit.text)
		position = edit.end
		changed = true
	}

	// Complete the range with unchanged base text for this side.
	result.WriteString(base[position:end])
	return result.String(), changed
}
