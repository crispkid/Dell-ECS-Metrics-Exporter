// Package releasepack creates deterministic release archives.
package releasepack

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Pack writes sourceDir as a deterministic gzip-compressed tar archive. Every
// entry is rooted below prefix; symlinks and non-regular files are rejected.
func Pack(sourceDir, outputPath, prefix string, modTime time.Time) (returnErr error) {
	if prefix == "" || prefix == "." || prefix == ".." || strings.Contains(prefix, "/") ||
		strings.Contains(prefix, `\`) {
		return fmt.Errorf("archive prefix must be one path segment")
	}
	info, err := os.Lstat(sourceDir)
	if err != nil {
		return fmt.Errorf("stat source directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("source must be a directory")
	}

	paths := make([]string, 0)
	err = filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == sourceDir {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed: %s", path)
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", path)
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk source directory: %w", err)
	}
	slices.SortFunc(paths, func(left, right string) int {
		leftRelative, _ := filepath.Rel(sourceDir, left)
		rightRelative, _ := filepath.Rel(sourceDir, right)
		return strings.Compare(filepath.ToSlash(leftRelative), filepath.ToSlash(rightRelative))
	})

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	temporary, err := os.CreateTemp(outputDir, ".release-archive-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary archive: %w", err)
	}
	temporaryPath := temporary.Name()
	temporaryClosed := false
	defer func() {
		if !temporaryClosed {
			if closeErr := temporary.Close(); returnErr == nil && closeErr != nil {
				returnErr = fmt.Errorf("close temporary archive: %w", closeErr)
			}
		}
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	gzipWriter := gzip.NewWriter(temporary)
	gzipWriter.Header.ModTime = time.Unix(0, 0).UTC()
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)

	writeHeader := func(name string, mode fs.FileMode, size int64, directory bool) error {
		typeFlag := byte(tar.TypeReg)
		if directory {
			typeFlag = tar.TypeDir
			if !strings.HasSuffix(name, "/") {
				name += "/"
			}
		}
		header := &tar.Header{
			Name:       name,
			Mode:       int64(mode.Perm()),
			Size:       size,
			Typeflag:   typeFlag,
			ModTime:    modTime.UTC(),
			AccessTime: time.Time{},
			ChangeTime: time.Time{},
			Uid:        0,
			Gid:        0,
			Uname:      "root",
			Gname:      "root",
			Format:     tar.FormatPAX,
		}
		return tarWriter.WriteHeader(header)
	}

	if err := writeHeader(prefix, 0o755, 0, true); err != nil {
		return fmt.Errorf("write root header: %w", err)
	}
	for _, path := range paths {
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("resolve relative archive path: %w", err)
		}
		name := prefix + "/" + filepath.ToSlash(relative)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat archive entry: %w", err)
		}
		if info.IsDir() {
			if err := writeHeader(name, 0o755, 0, true); err != nil {
				return fmt.Errorf("write directory header: %w", err)
			}
			continue
		}
		mode := fs.FileMode(0o644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		if err := writeHeader(name, mode, info.Size(), false); err != nil {
			return fmt.Errorf("write file header: %w", err)
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open archive entry: %w", err)
		}
		_, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		if copyErr != nil {
			return fmt.Errorf("copy archive entry: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close archive entry: %w", closeErr)
		}
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary archive: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary archive: %w", err)
	}
	temporaryClosed = true
	if err := os.Rename(temporaryPath, outputPath); err != nil {
		return fmt.Errorf("replace release archive: %w", err)
	}
	return nil
}
