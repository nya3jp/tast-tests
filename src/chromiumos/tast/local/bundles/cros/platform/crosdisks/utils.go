// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides a series of tests to verify CrosDisks'
// D-Bus API behavior.
package crosdisks

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

// The user which operates on files.
const chronos = "chronos"
const chronosUID = 1000
const chronosGID = 1000

// mount is a convenience wrapper for mounting with CrosDisks.
func mount(ctx context.Context, cd *crosdisks.CrosDisks, source, fsType string, options []string) (m crosdisks.MountCompleted, err error) {
	testing.ContextLogf(ctx, "Mounting %q as %q with options %q", source, fsType, options)
	m, err = cd.MountAndWaitForCompletion(ctx, source, fsType, options)
	if err != nil {
		err = errors.Wrap(err, "failed to invoke mount")
		return
	}
	testing.ContextLogf(ctx, "Mount completed with status %d", m.Status)
	if m.SourcePath != source {
		err = errors.Errorf("unexpected source_path: got %q; want %q", m.SourcePath, source)
	}
	return
}

// withMountDo mounts the specified source and if it succeeds calls the provided function, cleaning up the mount afterwards.
func withMountDo(ctx context.Context, cd *crosdisks.CrosDisks, source, fsType string, options []string, f func(ctx context.Context, mountPath string) error) (err error) {
	ctxForUnmount := ctx
	ctx, unmount := ctxutil.Shorten(ctx, time.Second*5)
	defer unmount()

	m, err := mount(ctx, cd, source, fsType, options)
	if err != nil {
		return err
	}

	if m.Status != 0 {
		return errors.Errorf("unexpected mount status: got %d; want %d", m.Status, 0)
	}
	defer func() {
		status, e := cd.Unmount(ctxForUnmount, m.MountPath, []string{})
		if e != nil {
			testing.ContextLogf(ctxForUnmount, "Could not invoke unmount %q: %v", m.MountPath, e)
			if err == nil {
				err = errors.Wrapf(e, "could not invoke unmount %q", m.MountPath)
			}
			return
		}
		if status != 0 {
			testing.ContextLogf(ctxForUnmount, "Failed to unmount %q: status %d", m.MountPath, status)
			if err == nil {
				err = errors.Wrapf(e, "failed to unmount %q: status %d", m.MountPath, status)
			}
		} else {
			if _, e := os.Stat(m.MountPath); e == nil {
				testing.ContextLogf(ctxForUnmount, "Mount point directory %q still present", m.MountPath)
				if err == nil {
					err = errors.Errorf("mount point directory %q still present", m.MountPath)
				}
			}
		}
	}()

	return f(ctx, m.MountPath)
}

// verifyMountStatus checks that mounting yields the expected status.
func verifyMountStatus(ctx context.Context, cd *crosdisks.CrosDisks, source, fsType string, options []string, expectedStatus uint32) error {
	m, err := mount(ctx, cd, source, fsType, options)
	if err != nil {
		return errors.Wrapf(err, "failed to invoke mount for %q", source)
	}
	if m.Status == 0 {
		defer cd.Unmount(ctx, m.MountPath, nil)
	}
	if m.Status != expectedStatus {
		return errors.Errorf("unexpected mount status for %q; got %d want %d", source, m.Status, expectedStatus)
	}
	return nil
}

// FileItem represents expectation for a file.
type FileItem struct {
	Mtime int64
	Data  []byte
}

// DirectoryContents maps from relative file names to properties of the file.
type DirectoryContents map[string]FileItem

// listDirectoryRecursively lists all files in a directory and its subdirectories.
func listDirectoryRecursively(rootDir string) (items DirectoryContents, err error) {
	dirs := []string{""}
	items = make(DirectoryContents)
	for len(dirs) > 0 {
		dir := dirs[0]
		dirs = dirs[1:]
		entries, err := ioutil.ReadDir(filepath.Join(rootDir, dir))
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			// Create an "empty dir" node.
			items[dir+"/"] = FileItem{}
		} else {
			for _, entry := range entries {
				relPath := filepath.Join(dir, entry.Name())
				if entry.IsDir() {
					dirs = append(dirs, relPath)
				} else {
					s, err := os.Stat(filepath.Join(rootDir, relPath))
					if err != nil {
						return nil, err
					}
					items[relPath] = FileItem{Mtime: s.ModTime().Unix()}
				}
			}
		}
	}
	return
}

// diffKeys calculates set(m1)-set(m2) and set(m2)-set(m1).
// If someone knows more idiomatic/shorter way of doing this in go - suggestions are welcome.
func diffKeys(m1, m2 DirectoryContents) (extra, missing []string) {
	extra = make([]string, 0)
	missing = make([]string, 0)
	for k := range m1 {
		_, ok := m2[k]
		if !ok {
			extra = append(extra, k)
		}
	}
	for k := range m2 {
		_, ok := m1[k]
		if !ok {
			missing = append(missing, k)
		}
	}
	return
}

// verifyThatKeysMatch checks that keys in both maps are same.
func verifyThatKeysMatch(ctx context.Context, actual, expected DirectoryContents) error {
	extra, missing := diffKeys(actual, expected)
	for _, v := range extra {
		testing.ContextLogf(ctx, "Extra item %q", v)
	}
	for _, v := range missing {
		testing.ContextLogf(ctx, "Missing item %q", v)
	}
	if len(extra) > 0 || len(missing) > 0 {
		return errors.Errorf("%d extra and %d missing items", len(extra), len(missing))
	}
	return nil
}

// verifyDirectoryContents recursively compares directory with the expectation and fails if there is a mismatch.
func verifyDirectoryContents(ctx context.Context, dir string, expectedContent DirectoryContents) error {
	testing.ContextLogf(ctx, "Listing items in %q", dir)
	files, err := listDirectoryRecursively(dir)
	if err != nil {
		return errors.Wrapf(err, "cannot list items in %q", dir)
	}
	if err := verifyThatKeysMatch(ctx, files, expectedContent); err != nil {
		return errors.Wrapf(err, "contents mismatch in %q", dir)
	}
	for k, v := range expectedContent {
		if v.Mtime != 0 {
			f := files[k]
			if f.Mtime != v.Mtime {
				return errors.Errorf("mtime of file %q does not match: got %d, want %d", k, f.Mtime, v.Mtime)
			}
		}

		if v.Data != nil {
			data, err := ioutil.ReadFile(filepath.Join(dir, k))
			if err != nil {
				return errors.Wrapf(err, "could not read file %q", k)
			}
			if bytes.Compare(v.Data, data) != 0 {
				return errors.Errorf("content of file %q does not match expected one", k)
			}
		}
	}

	return nil
}

// execAsUser runs a command as the |user|.
func execAsUser(ctx context.Context, user string, command []string) error {
	args := append([]string{"-u", user, "--"}, command...)
	if err := testexec.CommandContext(ctx, "sudo", args...).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "could not run %q as user %q", strings.Join(command, " "), user)
	}
	return nil
}
