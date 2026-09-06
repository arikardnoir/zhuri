// Package walk collects the list of files zhuri should process, applying
// the safety rules that keep a batch run from doing something surprising:
// no following symlinks, no touching special files, no reading giant files
// into memory just to skip them.
package walk

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SkipReason explains why a candidate path was left out of the batch.
type SkipReason string

const (
	SkipSymlink  SkipReason = "symlink"
	SkipSpecial  SkipReason = "ficheiro especial"
	SkipExt      SkipReason = "extensão não incluída"
	SkipTooLarge SkipReason = "excede o limite de tamanho"
)

// Skipped records one path that was not included, and why.
type Skipped struct {
	Path   string
	Reason SkipReason
	Size   int64 // only meaningful for SkipTooLarge
}

// Options controls how Collect walks the given paths.
type Options struct {
	// Recursive makes directories be walked all the way down. Without it,
	// only the immediate files of a given directory are considered.
	Recursive bool
	// MaxSize is the largest file (in bytes) that will be included. Zero
	// means unlimited.
	MaxSize int64
	// Extensions, when non-empty, restricts directory-discovered files to
	// these lowercase extensions (without the leading dot). Files passed
	// explicitly as arguments are always included regardless of extension.
	Extensions map[string]bool
}

// Collect resolves paths (files and/or directories) into the concrete list
// of regular files zhuri should process, plus everything that was skipped
// along the way.
func Collect(paths []string, opts Options) ([]string, []Skipped, error) {
	var files []string
	var skipped []Skipped
	seen := map[string]bool{}

	for _, p := range paths {
		lst, err := os.Lstat(p)
		if err != nil {
			return files, skipped, err
		}

		if lst.Mode()&os.ModeSymlink != 0 {
			skipped = append(skipped, Skipped{Path: p, Reason: SkipSymlink})
			continue
		}

		if lst.IsDir() {
			if opts.Recursive {
				if err := walkRecursive(p, opts, &files, &skipped, seen); err != nil {
					return files, skipped, err
				}
			} else {
				if err := walkTop(p, opts, &files, &skipped, seen); err != nil {
					return files, skipped, err
				}
			}
			continue
		}

		if !lst.Mode().IsRegular() {
			skipped = append(skipped, Skipped{Path: p, Reason: SkipSpecial})
			continue
		}

		addFile(p, lst, opts, &files, &skipped, seen, true)
	}

	return files, skipped, nil
}

func walkTop(dir string, opts Options, files *[]string, skipped *[]Skipped, seen map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.Type()&fs.ModeSymlink != 0 {
			*skipped = append(*skipped, Skipped{Path: path, Reason: SkipSymlink})
			continue
		}
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			*skipped = append(*skipped, Skipped{Path: path, Reason: SkipSpecial})
			continue
		}
		if !info.Mode().IsRegular() {
			*skipped = append(*skipped, Skipped{Path: path, Reason: SkipSpecial})
			continue
		}
		addFile(path, info, opts, files, skipped, seen, false)
	}
	return nil
}

func walkRecursive(root string, opts Options, files *[]string, skipped *[]Skipped, seen map[string]bool) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			*skipped = append(*skipped, Skipped{Path: path, Reason: SkipSpecial})
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			*skipped = append(*skipped, Skipped{Path: path, Reason: SkipSymlink})
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			*skipped = append(*skipped, Skipped{Path: path, Reason: SkipSpecial})
			return nil
		}
		if !info.Mode().IsRegular() {
			*skipped = append(*skipped, Skipped{Path: path, Reason: SkipSpecial})
			return nil
		}
		addFile(path, info, opts, files, skipped, seen, false)
		return nil
	})
}

func addFile(path string, info fs.FileInfo, opts Options, files *[]string, skipped *[]Skipped, seen map[string]bool, forceInclude bool) {
	if seen[path] {
		return
	}

	if !forceInclude && len(opts.Extensions) > 0 {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		if !opts.Extensions[ext] {
			*skipped = append(*skipped, Skipped{Path: path, Reason: SkipExt})
			return
		}
	}

	if opts.MaxSize > 0 && info.Size() > opts.MaxSize {
		*skipped = append(*skipped, Skipped{Path: path, Reason: SkipTooLarge, Size: info.Size()})
		return
	}

	seen[path] = true
	*files = append(*files, path)
}
