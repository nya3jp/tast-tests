// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filesnapshot provides functions that store/restore the snapshot (i.e. the content) of important files during integration test.
package filesnapshot

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"chromiumos/tast/errors"
)

// snapshotValue defines the a file snapshot, including the file content, permission, and its urser and group IDs.
type snapshotValue struct {
	content  []byte
	mode     os.FileMode
	uid, gid int
}

// Snapshot mentions the lookup table from the filename to the corresponding stored snapshot, and it supports the operations that are defined below.
type Snapshot struct {
	table map[string]snapshotValue
}

// NewSnapshot creates a new Snapshot of which lookup table is initialized.
func NewSnapshot() *Snapshot {
	return &Snapshot{make(map[string]snapshotValue)}
}

// Save reads the snapshot of filename, and stores it associated with the filename. If the snapshot is been saved before, overrides it.
func (s *Snapshot) Save(filename string) error {
	if !filepath.IsAbs(filename) {
		return errors.New("not an absolute path")
	}
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "failed to read file")
	}
	stat, err := os.Stat(filename)
	if err != nil {
		return errors.Wrap(err, "failed to get file stat")
	}

	rawStat, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("failed to get raw stat structure")
	}
	s.table[filename] = snapshotValue{content, stat.Mode(), int(rawStat.Uid), int(rawStat.Gid)}
	return nil
}

// Restore restores the snapshot of "filename". Note that the snapshot entry is not removed after the operation.
func (s *Snapshot) Restore(filename string) error {
	val, ok := s.table[filename]
	if !ok {
		return errors.Errorf("snapshot of path %s not found", filename)
	}
	if err := ioutil.WriteFile(filename, val.content, val.mode); err != nil {
		return errors.Wrap(err, "failed to restore file content")
	}
	// Always set the permission again; necessary when the file exist already when calling WriteFile.
	if err := os.Chmod(filename, val.mode); err != nil {
		return errors.Wrap(err, "failed to restore file permission")
	}
	if err := os.Chown(filename, val.uid, val.gid); err != nil {
		return errors.Wrap(err, "failed to restore file ownership")
	}
	return nil
}

// Remove removes the entry of filename from the snapshot lookup table; if the snapshot doesn't exist, performs no-ops.
func (s *Snapshot) Remove(filename string) {
	delete(s.table, filename)
}

// Stash stores the snapshot and delete the file.
func (s *Snapshot) Stash(filename string) error {
	if err := s.Save(filename); err != nil {
		return errors.Wrap(err, "failed to take a snapshot")
	}
	if err := os.Remove(filename); err != nil {
		return errors.Wrap(err, "failed to remove the file")
	}
	return nil
}

// Pop restores and the file content and delete the snapshot for restoration as well.
func (s *Snapshot) Pop(filename string) error {
	if err := s.Restore(filename); err != nil {
		return errors.Wrap(err, "failed to restore from snapshot")
	}
	s.Remove(filename)
	return nil
}
