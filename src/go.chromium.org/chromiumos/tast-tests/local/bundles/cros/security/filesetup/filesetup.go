// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filesetup provides file-related utility functions for security tests.
//
// All of the functions in this package panic on error.
// They are intended to be used to set up environments for testing, not to perform test assertions.
package filesetup

import (
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/sys/unix"
)

// CreateDir creates a directory at path owned by uid and with the supplied mode.
// Panics on error.
func CreateDir(path string, uid int, mode os.FileMode) {
	if err := os.Mkdir(path, mode); err != nil {
		panic(fmt.Sprintf("Failed to create %v: %v", path, err))
	}
	if err := os.Chown(path, uid, 0); err != nil {
		panic(fmt.Sprintf("Failed to chown %v to %v: %v", path, uid, err))
	}
	if err := os.Chmod(path, mode); err != nil {
		panic(fmt.Sprintf("Failed to chmod %v to %#o: %v", path, mode, err))
	}
}

// CreateFile creates a file at path containing data.
// The file will be owned by uid and will have the supplied mode.
// Panics on error.
func CreateFile(path, data string, uid int, mode os.FileMode) {
	if err := ioutil.WriteFile(path, []byte(data), mode); err != nil {
		panic(fmt.Sprintf("Failed to create %v containing %q: %v", path, data, err))
	}
	if err := os.Chown(path, uid, 0); err != nil {
		panic(fmt.Sprintf("Failed to chown %v to %v: %v", path, uid, err))
	}
	if err := os.Chmod(path, mode); err != nil {
		panic(fmt.Sprintf("Failed to chmod %v to %#o: %v", path, mode, err))
	}
}

// CreateSymlink creates a new symbolic link at newname pointing at target oldname.
// The symbolic link will be owned by uid.
// Panics on error.
func CreateSymlink(oldname, newname string, uid int) {
	if err := os.Symlink(oldname, newname); err != nil {
		panic(fmt.Sprintf("Failed to create %v -> %v symlink: %v", newname, oldname, err))
	}
	// Use Lchown to change the ownership of the symbolic link itself rather than the target.
	if err := os.Lchown(newname, uid, 0); err != nil {
		panic(fmt.Sprintf("Failed to chown %v to %v: %v", newname, uid, err))
	}
}

// ReadOnlyRootPartition returns true if the root partition is mounted read-only.
// This can be called by tests that inspect filesystem permissions: a false return value
// indicates that rootfs verification is disabled and that testing is likely to be unreliable.
func ReadOnlyRootPartition() (bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs("/", &st); err != nil {
		return false, err
	}
	return st.Flags&unix.ST_RDONLY != 0, nil
}
