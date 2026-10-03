package git

import (
	"io"

	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/utils/diff"
	"github.com/go-git/go-git/v6/utils/merkletrie"
)

// CherryPick applies each commit relative to its first parent onto HEAD.
// Independent changes at HEAD survive; the strategy chooses conflicting edits.
// A root commit is applied relative to an empty tree.
func (w *Worktree) CherryPick(commitOpts *CommitOptions, ortStrategyOption OrtMergeStrategyOption, commits ...*object.Commit) error {
	// Require commit options before changing the index or worktree.
	if commitOpts == nil {
		return ErrCannotCherryPickWithoutCommitOptions
	}

	// Load the checkout configuration before opening the reusable filesystem.
	cfg, err := w.r.Config()
	if err != nil {
		return err
	}

	// Materialise changes through the same validating filesystem and
	// checkout path as reset/checkout, so cherry-pick shares their
	// leading-symlink handling, mode awareness (symlinks, exec bits,
	// CRLF) and root reuse instead of writing raw bytes via Create.
	fs, closeFS := w.reusableRootFS()
	defer closeFS()

	// Apply each parent-relative change set, then advance HEAD with its commit.
	for _, commit := range commits {
		// Resolve the current tree for the three-way comparison.
		headRef, err := w.r.Head()
		if err != nil {
			return err
		}
		headCommit, err := w.r.CommitObject(headRef.Hash())
		if err != nil {
			return err
		}
		currentTree, err := headCommit.Tree()
		if err != nil {
			return err
		}

		// Use the picked commit's first parent as the merge base.
		var baseTree *object.Tree
		if len(commit.ParentHashes) != 0 {
			parent, err := commit.Parent(0)
			if err != nil {
				return err
			}
			baseTree, err = parent.Tree()
			if err != nil {
				return err
			}
		}

		// Compare both sides with the base, retaining independent HEAD edits.
		commitTree, err := commit.Tree()
		if err != nil {
			return err
		}

		// Resolve the picked changes against the current HEAD tree.
		changes, err := w.cherryPickChanges(baseTree, currentTree, commitTree, ortStrategyOption)
		if err != nil {
			return err
		}

		// Materialize only changes introduced by the picked commit.
		for _, change := range changes {
			// Resolve the file operation before touching the filesystem.
			action, err := change.Action()
			if err != nil {
				return err
			}

			// Share checkout validation and mode handling with reset.
			switch action {
			case merkletrie.Delete:
				if _, err := w.Remove(change.From.Name); err != nil {
					return err
				}
			case merkletrie.Insert, merkletrie.Modify:
				_, to, err := change.Files()
				if err != nil {
					return err
				}
				if to == nil {
					continue
				}
				// change.Files names the *File after the tree leaf. The
				// worktree write needs the full path so it lands at the
				// right location and is validated by the wrapper.
				to.Name = change.To.Name
				if err := w.checkoutFile(cfg, fs, to); err != nil {
					return err
				}
				if _, err := w.Add(to.Name); err != nil {
					return err
				}
			}
		}

		// Preserve the picked author and attach the new commit to current HEAD.
		_, err = w.Commit(commit.Message, &CommitOptions{
			Author:            &commit.Author,
			Committer:         commitOpts.Committer,
			Signer:            commitOpts.Signer,
			AllowEmptyCommits: commitOpts.AllowEmptyCommits,
		})
		if err != nil {
			return err
		}
	}

	// Complete the ordered cherry-pick sequence.
	return nil
}

// cherryPickChanges merges tree changes relative to the picked parent. Paths
// changed only at HEAD stay intact, and the strategy resolves overlapping edits.
func (w *Worktree) cherryPickChanges(base, ours, theirs *object.Tree, strategy OrtMergeStrategyOption) (object.Changes, error) {
	// Index the changes already present at HEAD relative to the same base.
	oursChanges, err := object.DiffTree(base, ours)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]*object.Change, len(oursChanges))
	for _, change := range oursChanges {
		name := change.To.Name
		if name == "" {
			name = change.From.Name
		}
		byPath[name] = change
	}

	// Resolve only paths introduced, modified or removed by the picked commit.
	changes, err := object.DiffTree(base, theirs)
	if err != nil {
		return nil, err
	}
	merged := make(object.Changes, 0, len(changes))
	for _, change := range changes {
		// Locate the corresponding HEAD change, including deleted paths.
		name := change.To.Name
		if name == "" {
			name = change.From.Name
		}
		ourChange := byPath[name]
		if ourChange == nil {
			merged = append(merged, change)
			continue
		}
		if ourChange.To.TreeEntry == change.To.TreeEntry {
			continue
		}

		// Resolve content and modes for files retained by both sides.
		if ourChange.To.TreeEntry.Mode.IsFile() && change.To.TreeEntry.Mode.IsFile() {
			resolved, err := w.mergeCherryPickFile(change, ourChange, strategy)
			if err != nil {
				return nil, err
			}
			if resolved.To.TreeEntry != ourChange.To.TreeEntry {
				merged = append(merged, resolved)
			}
			continue
		}

		// Retain HEAD on a conflicting deletion when the caller chooses ours.
		if strategy != OursMergeStrategy {
			merged = append(merged, change)
		}
	}

	// Return the merged changes without mutating the worktree or index.
	return merged, nil
}

// mergeCherryPickFile combines independent content and mode changes. Binary
// files and symlinks use the selected side when both sides changed their bytes.
func (w *Worktree) mergeCherryPickFile(theirChange, ourChange *object.Change, strategy OrtMergeStrategyOption) (*object.Change, error) {
	// Resolve the mode separately from content, using the parent as the base.
	base := theirChange.From.TreeEntry
	ours := ourChange.To.TreeEntry
	theirs := theirChange.To.TreeEntry
	result := *theirChange
	result.From = ourChange.To
	if theirs.Mode == base.Mode || strategy == OursMergeStrategy && ours.Mode != base.Mode {
		result.To.TreeEntry.Mode = ours.Mode
	}

	// Combine independent blob changes before handling overlapping text edits.
	switch {
	case theirs.Hash == base.Hash || theirs.Hash == ours.Hash:
		result.To.TreeEntry.Hash = ours.Hash
		return &result, nil
	case ours.Hash == base.Hash:
		return &result, nil
	}

	// Load both blobs and the optional base for an add/add conflict.
	ourFile, err := ourChange.To.Tree.TreeEntryFile(&ours)
	if err != nil {
		return nil, err
	}
	theirFile, err := theirChange.To.Tree.TreeEntryFile(&theirs)
	if err != nil {
		return nil, err
	}
	var baseText string
	if !base.Hash.IsZero() {
		baseFile, err := theirChange.From.Tree.TreeEntryFile(&base)
		if err != nil {
			return nil, err
		}
		baseText, err = baseFile.Contents()
		if err != nil {
			return nil, err
		}
	}

	// Symlink targets and binary blobs must be chosen as whole values.
	for _, file := range []*object.File{ourFile, theirFile} {
		binary, err := file.IsBinary()
		if err != nil {
			return nil, err
		}
		if binary || file.Mode == filemode.Symlink {
			if strategy == OursMergeStrategy {
				result.To.TreeEntry.Hash = ours.Hash
			}
			return &result, nil
		}
	}

	// Merge text relative to the base, choosing only overlapping edits by side.
	ourText, err := ourFile.Contents()
	if err != nil {
		return nil, err
	}
	theirText, err := theirFile.Contents()
	if err != nil {
		return nil, err
	}
	text := diff.Merge(baseText, ourText, theirText, strategy == OursMergeStrategy)

	// Encode the merged text into a blob for the existing checkout path.
	blob := w.r.Storer.NewEncodedObject()
	blob.SetType(plumbing.BlobObject)
	blob.SetSize(int64(len(text)))
	writer, err := blob.Writer()
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(writer, text); err != nil {
		_ = writer.Close() // Preserve the write error when cleanup also fails.
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	// Store the complete blob before returning its merged tree entry.
	hash, err := w.r.Storer.SetEncodedObject(blob)
	if err != nil {
		return nil, err
	}
	result.To.TreeEntry.Hash = hash
	return &result, nil
}
