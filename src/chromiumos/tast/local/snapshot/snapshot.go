// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package snapshot provides functions that store/restore the snapshot(i.e. the content) of important files during integration test. It also provides a system-level (i.e. set up by tast framework) snaphot manager for test items to use.
package snapshot

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"chromiumos/tast/errors"
)

var mgrForSys *Manager

func init() {
	mgrForSys = NewManager()
}

// SystemLevelManager returns the system-level Manager, which stores the important snapshot by in tast framework.
func SystemLevelManager() *Manager {
	return mgrForSys
}

// snapshotKey defines the key used to look up the contents of a snapshot.
type snapshotKey struct {
	filename, label string
}

// snapshotValue defiens the a file snapshot, including the file content, permission, and its urser and group IDs.
type snapshotValue struct {
	content  []byte
	mode     os.FileMode
	UID, GID int
}

// Manager mentions the lookup table from snapshotKey to snapshotValue, and it supports the operations that are defined below.
type Manager struct {
	table map[snapshotKey]snapshotValue
}

// NewManager creates a new Manager of which lookup table is initialized.
func NewManager() *Manager {
	return &Manager{make(map[snapshotKey]snapshotValue)}
}

// Store reads the snapshot of filename, and stores it associated with the filename/label pair.
func (sm *Manager) Store(ctx context.Context, filename, label string) error {
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
	sm.table[snapshotKey{filename, label}] = snapshotValue{content, stat.Mode(), int(rawStat.Uid), int(rawStat.Gid)}
	return nil
}

// Restore restores the snapshot of filename/label pair to filename. if shallDelete is true, the snapshot is deleted from the lookup table.
func (sm *Manager) Restore(ctx context.Context, filename, label string, shallDelete bool) error {
	key := snapshotKey{filename, label}
	val, ok := sm.table[key]
	if !ok {
		return errors.Errorf("snapshot of path %s not found for label %s", filename, label)
	}
	if err := ioutil.WriteFile(filename, val.content, val.mode); err != nil {
		return errors.Wrap(err, "failed to restore file content")
	}
	// Always set the permission again; necessary when the file exist already when calling WriteFile.
	if err := os.Chmod(filename, val.mode); err != nil {
		return errors.Wrap(err, "failed to restore file permission")
	}
	if err := os.Chown(filename, val.UID, val.GID); err != nil {
		return errors.Wrap(err, "failed to restore file ownership")
	}
	if shallDelete {
		delete(sm.table, key)
	}
	return nil
}

// Stash stores the snapshot and delete the file.
func (sm *Manager) Stash(ctx context.Context, filename, label string) error {
	if err := sm.Store(ctx, filename, label); err != nil {
		return errors.Wrap(err, "failed to take a snapshot")
	}
	if err := os.Remove(filename); err != nil {
		return errors.Wrap(err, "failed to remove the file")
	}
	return nil
}

// Pop restores and the file content and delete the snapshot for restoration as well.
func (sm *Manager) Pop(ctx context.Context, filename, label string) error {
	return sm.Restore(ctx, filename, label, true)
}
