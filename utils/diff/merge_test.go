package diff

import "testing"

// TestMerge checks independent, identical and overlapping line edits against
// results from git merge-file with the corresponding conflict preference.
func TestMerge(t *testing.T) {
	// Cover insertions, replacements, deletions and connected conflicts.
	cases := []struct {
		name, base, ours, theirs, wantOurs, wantTheirs string
	}{
		{"independent", "one\ntwo\nthree\n", "ONE\ntwo\nthree\n", "one\ntwo\nTHREE\n", "ONE\ntwo\nTHREE\n", "ONE\ntwo\nTHREE\n"},
		{"conflict", "one\ntwo\nthree\n", "OURS\ntwo\nthree\n", "THEIRS\ntwo\nthree\n", "OURS\ntwo\nthree\n", "THEIRS\ntwo\nthree\n"},
		{"identical", "base\n", "same\n", "same\n", "same\n", "same\n"},
		{"add-add", "", "ours\n", "theirs\n", "ours\n", "theirs\n"},
		{"insertions", "one\ntwo\nthree\n", "before\none\ntwo\nthree\n", "one\ntwo\nthree\nafter\n", "before\none\ntwo\nthree\nafter\n", "before\none\ntwo\nthree\nafter\n"},
		{"deletion", "one\ntwo\nthree\n", "two\nthree\n", "one\ntwo\nTHREE\n", "two\nTHREE\n", "two\nTHREE\n"},
		{"adjacent", "one\ntwo\nthree\n", "ONE\ntwo\nthree\n", "one\nTWO\nthree\n", "ONE\ntwo\nthree\n", "one\nTWO\nthree\n"},
		{"no-final-newline", "one\ntwo\nthree", "ONE\ntwo\nthree", "one\ntwo\nTHREE", "ONE\ntwo\nTHREE", "ONE\ntwo\nTHREE"},
		{"overlap-chain", "a\nb\nc\nd\ne\nf\ng\n", "A\nc\nd\nE\ng\n", "a\nB\nf\ng\n", "A\nc\nd\nE\ng\n", "a\nB\nf\ng\n"},
	}

	// Both preferences must preserve every edit outside their conflict ranges.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Merge(tc.base, tc.ours, tc.theirs, true); got != tc.wantOurs {
				t.Fatalf("ours = %q, want %q", got, tc.wantOurs)
			}
			if got := Merge(tc.base, tc.ours, tc.theirs, false); got != tc.wantTheirs {
				t.Fatalf("theirs = %q, want %q", got, tc.wantTheirs)
			}
		})
	}
}
