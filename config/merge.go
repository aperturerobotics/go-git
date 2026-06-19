package config

import (
	"maps"

	format "github.com/go-git/go-git/v6/plumbing/format/config"
)

func merge(dst, src *Config) {
	mergeCore(&dst.Core, &src.Core)
	mergeUser(&dst.User, &src.User)
	mergeIdentity(&dst.Author, &src.Author)
	mergeIdentity(&dst.Committer, &src.Committer)

	if src.Tag.GpgSign.IsSet() {
		dst.Tag.GpgSign = src.Tag.GpgSign
	}
	if src.Commit.GpgSign.IsSet() {
		dst.Commit.GpgSign = src.Commit.GpgSign
	}

	if src.GPG.Format != "" {
		dst.GPG.Format = src.GPG.Format
	}
	if src.GPG.SSH.AllowedSignersFile != "" {
		dst.GPG.SSH.AllowedSignersFile = src.GPG.SSH.AllowedSignersFile
	}

	if src.Pack.Window != 0 {
		dst.Pack.Window = src.Pack.Window
	}
	if src.Pack.ReadReverseIndex {
		dst.Pack.ReadReverseIndex = true
	}
	if src.Pack.WriteReverseIndex {
		dst.Pack.WriteReverseIndex = true
	}

	if src.Index.SkipHash.IsSet() {
		dst.Index.SkipHash = src.Index.SkipHash
	}
	if src.Init.DefaultBranch != "" {
		dst.Init.DefaultBranch = src.Init.DefaultBranch
	}
	if src.UploadArchive.AllowUnreachable.IsSet() {
		dst.UploadArchive.AllowUnreachable = src.UploadArchive.AllowUnreachable
	}
	if src.Extensions.ObjectFormat != "" {
		dst.Extensions.ObjectFormat = src.Extensions.ObjectFormat
	}
	if src.Extensions.WorktreeConfig {
		dst.Extensions.WorktreeConfig = true
	}
	if src.Protocol.Version != 0 {
		dst.Protocol.Version = src.Protocol.Version
	}

	mergeMap(&dst.Remotes, src.Remotes)
	mergeMap(&dst.Submodules, src.Submodules)
	mergeMap(&dst.Branches, src.Branches)
	mergeMap(&dst.URLs, src.URLs)
}

func mergeCore(dst, src *struct {
	IsBare                  bool
	Worktree                string
	CommentChar             string
	RepositoryFormatVersion format.RepositoryFormatVersion
	AutoCRLF                string
	FileMode                bool
	HooksPath               string
	ProtectNTFS             OptBool
	ProtectHFS              OptBool
}) {
	if src.IsBare {
		dst.IsBare = true
	}
	if src.Worktree != "" {
		dst.Worktree = src.Worktree
	}
	if src.CommentChar != "" {
		dst.CommentChar = src.CommentChar
	}
	if src.RepositoryFormatVersion != "" {
		dst.RepositoryFormatVersion = src.RepositoryFormatVersion
	}
	if src.AutoCRLF != "" {
		dst.AutoCRLF = src.AutoCRLF
	}
	if src.FileMode {
		dst.FileMode = true
	}
	if src.HooksPath != "" {
		dst.HooksPath = src.HooksPath
	}
	if src.ProtectNTFS.IsSet() {
		dst.ProtectNTFS = src.ProtectNTFS
	}
	if src.ProtectHFS.IsSet() {
		dst.ProtectHFS = src.ProtectHFS
	}
}

func mergeUser(dst, src *user) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Email != "" {
		dst.Email = src.Email
	}
	if src.SigningKey != "" {
		dst.SigningKey = src.SigningKey
	}
}

func mergeIdentity(dst, src *struct {
	Name  string
	Email string
}) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Email != "" {
		dst.Email = src.Email
	}
}

func mergeMap[K comparable, V any](dst *map[K]V, src map[K]V) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[K]V)
	}
	maps.Copy(*dst, src)
}
