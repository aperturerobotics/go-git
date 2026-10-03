//go:build !goscript

package git

import (
	"testing"

	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/storage/memory"
)

// TestCherryPickPreservesMergedSibling checks that either strategy retains a
// fast-forwarded sibling while applying the picked sibling's independent file.
func TestCherryPickPreservesMergedSibling(t *testing.T) {
	// Exercise both conflict preferences on changes without a conflict.
	for _, strategy := range []OrtMergeStrategyOption{TheirsMergeStrategy, OursMergeStrategy} {
		t.Run(strategyName(strategy), func(t *testing.T) {
			// Open an in-memory repository for the sibling commits.
			fs := memfs.New()
			repo, err := Init(memory.NewStorage(), WithWorkTree(fs))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := repo.Close(); err != nil {
					t.Error(err)
				}
			})
			wt, err := repo.Worktree()
			if err != nil {
				t.Fatal(err)
			}

			// Commit both siblings over the same base tree.
			base := commitCherryPickFile(t, wt, "base.txt", "base\n")
			first := commitCherryPickFile(t, wt, "first.txt", "first\n")
			if err := wt.Reset(&ResetOptions{Commit: base, Mode: HardReset}); err != nil {
				t.Fatal(err)
			}
			second := commitCherryPickFile(t, wt, "second.txt", "second\n")

			// Fast-forward HEAD to the first sibling, then cherry-pick the second.
			if err := wt.Reset(&ResetOptions{Commit: base, Mode: HardReset}); err != nil {
				t.Fatal(err)
			}
			if err := repo.Merge(*plumbing.NewHashReference("refs/heads/first", first), MergeOptions{Strategy: FastForwardMerge}); err != nil {
				t.Fatal(err)
			}
			if err := wt.Reset(&ResetOptions{Commit: first, Mode: HardReset}); err != nil {
				t.Fatal(err)
			}
			picked, err := repo.CommitObject(second)
			if err != nil {
				t.Fatal(err)
			}
			if err := wt.CherryPick(&CommitOptions{Committer: defaultSignature()}, strategy, picked); err != nil {
				t.Fatal(err)
			}

			// Verify the cherry-picked commit descends from the integrated sibling.
			head, err := repo.Head()
			if err != nil {
				t.Fatal(err)
			}
			commit, err := repo.CommitObject(head.Hash())
			if err != nil {
				t.Fatal(err)
			}
			if len(commit.ParentHashes) != 1 || commit.ParentHashes[0] != first {
				t.Fatalf("parents = %v, want [%s]", commit.ParentHashes, first)
			}
			tree, err := commit.Tree()
			if err != nil {
				t.Fatal(err)
			}

			// Verify both siblings in the filesystem and committed tree.
			for name, want := range map[string]string{"base.txt": "base\n", "first.txt": "first\n", "second.txt": "second\n"} {
				data, err := util.ReadFile(fs, name)
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != want {
					t.Fatalf("%s = %q, want %q", name, data, want)
				}
				file, err := tree.File(name)
				if err != nil {
					t.Fatal(err)
				}
				contents, err := file.Contents()
				if err != nil {
					t.Fatal(err)
				}
				if contents != want {
					t.Fatalf("committed %s = %q, want %q", name, contents, want)
				}
			}

			// Verify the index and worktree agree after the cherry-pick.
			status, err := wt.Status()
			if err != nil {
				t.Fatal(err)
			}
			if !status.IsClean() {
				t.Fatalf("worktree is dirty: %s", status)
			}
		})
	}
}

// commitCherryPickFile stages and commits one file through the worktree API.
func commitCherryPickFile(t *testing.T, wt *Worktree, name, contents string) plumbing.Hash {
	// Write and stage the file before creating its commit.
	t.Helper()
	if err := util.WriteFile(wt.Filesystem(), name, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add(name); err != nil {
		t.Fatal(err)
	}

	// Give fixture commits a deterministic author and timestamp.
	hash, err := wt.Commit(name, &CommitOptions{Author: defaultSignature()})
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

// strategyName describes a conflict preference in test output.
func strategyName(strategy OrtMergeStrategyOption) string {
	if strategy == OursMergeStrategy {
		return "ours"
	}
	return "theirs"
}

// TestCherryPickMergesFileEdits checks that a strategy selects conflicting lines
// while preserving independent edits to the same file.
func TestCherryPickMergesFileEdits(t *testing.T) {
	// Exercise independent and overlapping edits with each conflict preference.
	for _, strategy := range []OrtMergeStrategyOption{TheirsMergeStrategy, OursMergeStrategy} {
		t.Run(strategyName(strategy), func(t *testing.T) {
			// Open an in-memory repository for the conflicting sibling commits.
			fs := memfs.New()
			repo, err := Init(memory.NewStorage(), WithWorkTree(fs))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := repo.Close(); err != nil {
					t.Error(err)
				}
			})
			wt, err := repo.Worktree()
			if err != nil {
				t.Fatal(err)
			}

			// Commit both siblings over the same base tree.
			base := commitCherryPickFile(t, wt, "file.txt", "a\nb\nc\nd\ne\n")
			ours := commitCherryPickFile(t, wt, "file.txt", "OURS\nb\nours\nd\ne\n")
			if err := wt.Reset(&ResetOptions{Commit: base, Mode: HardReset}); err != nil {
				t.Fatal(err)
			}
			theirs := commitCherryPickFile(t, wt, "file.txt", "THEIRS\nb\nc\nd\ntheirs\n")

			// Apply the picked sibling onto the current sibling.
			if err := wt.Reset(&ResetOptions{Commit: ours, Mode: HardReset}); err != nil {
				t.Fatal(err)
			}
			picked, err := repo.CommitObject(theirs)
			if err != nil {
				t.Fatal(err)
			}
			if err := wt.CherryPick(&CommitOptions{Committer: defaultSignature()}, strategy, picked); err != nil {
				t.Fatal(err)
			}

			// Only the first line conflicts; each sibling's later edit survives.
			want := "THEIRS\nb\nours\nd\ntheirs\n"
			if strategy == OursMergeStrategy {
				want = "OURS\nb\nours\nd\ntheirs\n"
			}
			data, err := util.ReadFile(fs, "file.txt")
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != want {
				t.Fatalf("file.txt = %q, want %q", data, want)
			}

			// Verify the index and worktree agree after the cherry-pick.
			status, err := wt.Status()
			if err != nil {
				t.Fatal(err)
			}
			if !status.IsClean() {
				t.Fatalf("worktree is dirty: %s", status)
			}
		})
	}
}

// TestCherryPickRootCommit checks that the empty-tree base preserves current
// files while applying files from an unrelated root commit.
func TestCherryPickRootCommit(t *testing.T) {
	// Commit the current root in an in-memory repository.
	store := memory.NewStorage()
	fs := memfs.New()
	repo, err := Init(store, WithWorkTree(fs))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := repo.Close(); err != nil {
			t.Error(err)
		}
	})
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	commitCherryPickFile(t, wt, "ours.txt", "ours\n")

	// Commit an independent root in its own repository.
	rootRepo, err := Init(memory.NewStorage(), WithWorkTree(memfs.New()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rootRepo.Close(); err != nil {
			t.Error(err)
		}
	})
	rootWT, err := rootRepo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	root := commitCherryPickFile(t, rootWT, "root.txt", "root\n")

	// Import the unrelated root's objects without changing HEAD or the index.
	objects, err := rootRepo.Storer.IterEncodedObjects(plumbing.AnyObject)
	if err != nil {
		t.Fatal(err)
	}
	if err := objects.ForEach(func(obj plumbing.EncodedObject) error {
		_, err := repo.Storer.SetEncodedObject(obj)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	// Apply the unrelated root on top of the current commit.
	picked, err := repo.CommitObject(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(picked.ParentHashes) != 0 {
		t.Fatalf("picked commit has parents: %v", picked.ParentHashes)
	}
	if err := wt.CherryPick(&CommitOptions{Committer: defaultSignature()}, TheirsMergeStrategy, picked); err != nil {
		t.Fatal(err)
	}

	// Both root files must remain in the resulting worktree.
	for name, want := range map[string]string{"ours.txt": "ours\n", "root.txt": "root\n"} {
		data, err := util.ReadFile(fs, name)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("%s = %q, want %q", name, data, want)
		}
	}
}
