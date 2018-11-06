// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filesetup provides file-related utility functions for security tests.
//
// All of the functions in this package take testing.State arguments and report fatal errors.
// They are intended to be used to set up environments for testing, not to perform test assertions.
package filesetup

import (
	"io/ioutil"
	"os"
	"os/user"
	"strconv"

	"chromiumos/tast/testing"
)

// GetUID returns the UID corresponding to username, which must exist.
func GetUID(s *testing.State, username string) int {
	u, err := user.Lookup(username)
	if err != nil {
		s.Fatalf("Failed to look up user %q: %v", username, err)
	}
	uid, err := strconv.ParseInt(u.Uid, 10, 64)
	if err != nil {
		s.Fatalf("Failed to parse UID %q for user %q: %v", u.Uid, username, err)
	}
	return int(uid)
}

// CreateFile creates a file at path containing data.
// The file will be owned by uid and will have the supplied mode.
func CreateFile(s *testing.State, path, data string, uid int, mode os.FileMode) {
	if err := ioutil.WriteFile(path, []byte(data), mode); err != nil {
		s.Fatalf("Failed to create %v containing %q: %v", path, data, err)
	}
	if err := os.Chown(path, uid, 0); err != nil {
		s.Fatalf("Failed to chown %v to %v: %v", path, uid, err)
	}
}

// CreateSymlink creates a new symbolic link at newname pointing at target oldname.
// The symbolic link will be owned by uid.
func CreateSymlink(s *testing.State, oldname, newname string, uid int) {
	if err := os.Symlink(oldname, newname); err != nil {
		s.Fatalf("Failed to create %v -> %v symlink: %v", newname, oldname, err)
	}
	// Use Lchown to change the ownership of the symbolic link itself rather than the target.
	if err := os.Lchown(newname, uid, 0); err != nil {
		s.Fatalf("Failed to chown %v to %v: %v", newname, uid, err)
	}
}
