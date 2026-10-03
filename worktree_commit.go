package git

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	"github.com/go-git/go-git/v6/plumbing/format/index"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/storage"
	"github.com/go-git/go-git/v6/utils/trace"
	"github.com/go-git/go-git/v6/x/plugin"
)

var (
	// ErrEmptyCommit occurs when a commit is attempted using a clean
	// working tree, with no changes to be committed.
	ErrEmptyCommit = errors.New("cannot create empty commit: clean working tree")
	// ErrCannotCherryPickWithoutCommitOptions indicates missing cherry-pick options.
	ErrCannotCherryPickWithoutCommitOptions = errors.New("cannot cherry-pick without commit options")

	// invalidCharactersRe strips characters prohibited in commit identities.
	// See https://git-scm.com/docs/git-commit#_commit_information.
	invalidCharactersRe = regexp.MustCompile(`[<>\n]`)
)

// Commit stores the current contents of the index in a new commit along with
// a log message from the user describing the changes.
func (w *Worktree) Commit(msg string, opts *CommitOptions) (plumbing.Hash, error) {
	// Measure commit latency when performance tracing is enabled.
	if trace.Performance.Enabled() {
		start := time.Now()
		defer func() {
			trace.Performance.Printf("performance: %.9f s: git command: git commit", time.Since(start).Seconds())
		}()
	}

	// Resolve commit options and parent defaults before staging changes.
	if err := opts.Validate(w.r); err != nil {
		return plumbing.ZeroHash, err
	}

	// Stage modifications and deletions requested by the caller.
	if opts.All {
		if err := w.autoAddModifiedAndDeleted(); err != nil {
			return plumbing.ZeroHash, err
		}
	}

	// Preserve the existing parent list when replacing the HEAD commit.
	if opts.Amend {
		head, err := w.r.Head()
		if err != nil {
			return plumbing.ZeroHash, err
		}
		headCommit, err := w.r.CommitObject(head.Hash())
		if err != nil {
			return plumbing.ZeroHash, err
		}

		// Reuse the amended commit's ancestry.
		opts.Parents = headCommit.ParentHashes
	}

	// Read the staged entries that define the new tree.
	idx, err := w.r.Storer.Index()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	// Reject an empty initial tree unless empty commits are enabled.
	if len(opts.Parents) == 0 && len(idx.Entries) == 0 && !opts.AllowEmptyCommits {
		return plumbing.ZeroHash, ErrEmptyCommit
	}

	// Build the tree from the index's blob hashes and file modes.
	h := &buildTreeHelper{
		fs: w.filesystem,
		s:  w.r.Storer,
	}

	// Persist the complete index tree before comparing it with the parent.
	treeHash, err := h.BuildTree(idx, opts)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	// Compare with the first parent to detect an unchanged tree.
	previousTree := plumbing.ZeroHash
	if len(opts.Parents) > 0 {
		parentCommit, err := w.r.CommitObject(opts.Parents[0])
		if err != nil {
			return plumbing.ZeroHash, err
		}
		previousTree = parentCommit.TreeHash
	}

	// Reject an unchanged tree unless empty commits are enabled.
	if treeHash == previousTree && !opts.AllowEmptyCommits {
		return plumbing.ZeroHash, ErrEmptyCommit
	}

	// Store the new commit before publishing it through HEAD.
	commit, err := w.buildCommitObject(msg, opts, treeHash)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	// Advance the branch or detached HEAD to the stored commit.
	return commit, w.updateHEAD(commit)
}

// autoAddModifiedAndDeleted stages tracked modifications and deletions.
func (w *Worktree) autoAddModifiedAndDeleted() error {
	// Load staging configuration before comparing the index and worktree.
	cfg, err := w.r.Config()
	if err != nil {
		return err
	}

	// Inspect tracked changes in the filesystem.
	s, err := w.Status()
	if err != nil {
		return err
	}

	// Load the index that will receive all tracked worktree changes.
	idx, err := w.r.Storer.Index()
	if err != nil {
		return err
	}

	// Stage only tracked modifications and deletions.
	for path, fs := range s {
		if fs.Worktree != Modified && fs.Worktree != Deleted {
			continue
		}

		// Apply the tracked path's new state to the index.
		if _, _, err := w.doAddFile(cfg, idx, s, path, nil); err != nil {
			return err
		}
	}

	// Publish the updated index once every tracked change is staged.
	return w.r.Storer.SetIndex(idx)
}

// updateHEAD advances the current branch or detached HEAD to commit.
func (w *Worktree) updateHEAD(commit plumbing.Hash) error {
	// Determine whether HEAD is detached or points to a branch.
	head, err := w.r.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}

	// Update the branch target for a symbolic HEAD.
	name := plumbing.HEAD
	if head.Type() != plumbing.HashReference {
		name = head.Target()
	}

	// Publish the new commit through the selected reference.
	ref := plumbing.NewHashReference(name, commit)
	return w.r.Storer.SetReference(ref)
}

// buildCommitObject stores a commit with sanitized identities and an optional signature.
func (w *Worktree) buildCommitObject(msg string, opts *CommitOptions, tree plumbing.Hash) (plumbing.Hash, error) {
	// Assemble the commit record from the tree and resolved options.
	commit := &object.Commit{
		Author:       w.sanitize(*opts.Author),
		Committer:    w.sanitize(*opts.Committer),
		Message:      msg,
		TreeHash:     tree,
		ParentHashes: opts.Parents,
	}

	// Resolve the configured signer when no explicit signer was supplied.
	signer := opts.Signer
	if signer == nil {
		cfg, err := w.r.ConfigScoped(config.SystemScope)
		if err == nil && cfg != nil && cfg.Commit.GpgSign.IsTrue() {
			// Use Has before Get so the key is not frozen when no plugin is
			// registered, allowing callers to register one later.
			if !plugin.Has(plugin.ObjectSigner()) {
				return plumbing.ZeroHash, fmt.Errorf("cannot auto-sign commit: disable commit.gpgSign or register an ObjectSigner plugin")
			}

			// Acquire the registered signer after confirming its availability.
			signer, err = plugin.Get(plugin.ObjectSigner())
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("get object signer: %w", err)
			}
		}
	}

	// Attach the signature before encoding the final commit object.
	if signer != nil {
		sig, err := signObject(signer, commit)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		commit.Signature = string(sig)
	}

	// Persist the complete commit in the repository object store.
	obj := w.r.Storer.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return w.r.Storer.SetEncodedObject(obj)
}

// sanitize removes characters Git prohibits in commit author and committer identities.
func (w *Worktree) sanitize(signature object.Signature) object.Signature {
	return object.Signature{
		Name:  invalidCharactersRe.ReplaceAllString(signature.Name, ""),
		Email: invalidCharactersRe.ReplaceAllString(signature.Email, ""),
		When:  signature.When,
	}
}

// buildTreeHelper converts a given index.Index file into multiple git objects
// reading the blobs from the given filesystem and creating the trees from the
// index structure. The created objects are pushed to a given Storer.
type buildTreeHelper struct {
	// fs is the worktree filesystem used by the commit builder.
	fs billy.Filesystem
	// s receives the encoded tree objects.
	s storage.Storer

	// trees holds the tree under construction for each directory path.
	trees map[string]*object.Tree
	// entries tracks paths already represented as entries.
	entries map[string]*object.TreeEntry
}

// BuildTree stores the index tree and returns its root hash.
func (h *buildTreeHelper) BuildTree(idx *index.Index, _ *CommitOptions) (plumbing.Hash, error) {
	// Initialize the root and path maps for this index.
	const rootNode = ""
	h.trees = map[string]*object.Tree{rootNode: {}}
	h.entries = map[string]*object.TreeEntry{}

	// Attach every indexed file to its directory tree.
	for _, e := range idx.Entries {
		if err := h.commitIndexEntry(e); err != nil {
			return plumbing.ZeroHash, err
		}
	}

	// Persist descendants before encoding the root tree.
	return h.copyTreeToStorageRecursive(rootNode, h.trees[rootNode])
}

// commitIndexEntry attaches one indexed path and its parent directories.
func (h *buildTreeHelper) commitIndexEntry(e *index.Entry) error {
	// Index entries with a zero hash point at no object. Tree.Encode
	// (through Tree.Validate) refuses to write them, and the pre-fsck
	// behavior of #1773 was to accept the entry but never reach a
	// healthy tree. Skip them here so the resulting tree is well-formed.
	if e.Hash.IsZero() {
		return nil
	}

	// Split the repository-relative path into directory and leaf components.
	parts := strings.Split(e.Name, "/")

	// Build each missing directory and the final indexed leaf.
	var fullpath string
	for _, part := range parts {
		parent := fullpath
		fullpath = path.Join(fullpath, part)

		// Attach the component to its parent tree.
		h.doBuildTree(e, parent, fullpath)
	}

	// Finish after the full indexed path is represented.
	return nil
}

// doBuildTree adds one missing directory or leaf to its parent tree.
func (h *buildTreeHelper) doBuildTree(e *index.Entry, parent, fullpath string) {
	// Reuse a directory tree already built for another indexed path.
	if _, ok := h.trees[fullpath]; ok {
		return
	}

	// Reuse a leaf entry already represented at this path.
	if _, ok := h.entries[fullpath]; ok {
		return
	}

	// Construct a tree entry for either the indexed leaf or its directory.
	te := object.TreeEntry{Name: path.Base(fullpath)}

	// Assign the leaf's blob and mode, or allocate its directory tree.
	if fullpath == e.Name {
		te.Mode = e.Mode
		te.Hash = e.Hash
	} else {
		te.Mode = filemode.Dir
		h.trees[fullpath] = &object.Tree{}
	}

	// Attach the new entry to its parent directory.
	h.trees[parent].Entries = append(h.trees[parent].Entries, te)
}

// sortableEntries orders tree entries using Git's directory suffix rule.
type sortableEntries []object.TreeEntry

// sortName includes the trailing slash that Git uses when ordering directories.
func (sortableEntries) sortName(te object.TreeEntry) string {
	if te.Mode == filemode.Dir {
		return te.Name + "/"
	}
	return te.Name
}

// Len returns the number of entries.
func (se sortableEntries) Len() int { return len(se) }

// Less compares entries in Git tree order.
func (se sortableEntries) Less(i, j int) bool { return se.sortName(se[i]) < se.sortName(se[j]) }

// Swap exchanges two entries during sorting.
func (se sortableEntries) Swap(i, j int) { se[i], se[j] = se[j], se[i] }

// copyTreeToStorageRecursive stores descendants and returns the encoded tree hash.
func (h *buildTreeHelper) copyTreeToStorageRecursive(parent string, t *object.Tree) (plumbing.Hash, error) {
	// Sort entries in Git tree order before encoding descendants.
	sort.Sort(sortableEntries(t.Entries))
	for i, e := range t.Entries {
		if e.Mode != filemode.Dir {
			continue
		}

		// Store the child tree before recording its hash in the parent.
		path := path.Join(parent, e.Name)

		// Resolve the child hash through the same recursive store.
		var err error
		e.Hash, err = h.copyTreeToStorageRecursive(path, h.trees[path])
		if err != nil {
			return plumbing.ZeroHash, err
		}

		// Publish the stored child hash into its parent entry.
		t.Entries[i] = e
	}

	// Encode the parent with its complete set of child hashes.
	o := h.s.NewEncodedObject()
	if err := t.Encode(o); err != nil {
		return plumbing.ZeroHash, err
	}

	// Reuse a stored tree with identical bytes, or store the new object.
	hash := o.Hash()
	if h.s.HasEncodedObject(hash) == nil {
		return hash, nil
	}
	return h.s.SetEncodedObject(o)
}
