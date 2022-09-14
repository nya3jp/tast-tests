// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StatefulPartitionHardening,
		Desc: "Tests access behavior of symlinks and FIFOs",
		Contacts: []string{
			"jorgelo@chromium.org",  // Security team
			"ejcaruso@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func StatefulPartitionHardening(ctx context.Context, s *testing.State) {
	generateTempFilename := func(parent, fileType string) string {
		template := fmt.Sprintf("tast.security.StatefulPartitionHardening.%v.", fileType)
		tempFile, err := ioutil.TempFile(parent, template)
		if err != nil {
			s.Fatalf("Could not generate temp %v in %v", fileType, parent)
		}
		if err := tempFile.Close(); err != nil {
			s.Errorf("Failed to close %v: %v", tempFile.Name(), err)
		}
		if err := os.Remove(tempFile.Name()); err != nil {
			s.Errorf("Failed to remove %v: %v", tempFile.Name(), err)
		}
		return tempFile.Name()
	}

	expectAccess := func(path string, expected bool) {
		fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK, 0777)
		if err == nil {
			if err := unix.Close(fd); err != nil {
				s.Errorf("Failed to close FD %v: %v", fd, err)
			}
			if !expected {
				s.Errorf("Opening %v unexpectedly succeeded", path)
			}
		} else if expected {
			s.Errorf("Opening %v failed: %v", path, err)
		}
	}

	expectFIFOAccess := func(parent string, expected bool) {
		path := generateTempFilename(parent, "fifo")
		if err := unix.Mkfifo(path, 0666); err != nil {
			s.Fatalf("Failed to create FIFO at %v: %v", path, err)
		}
		defer os.Remove(path)
		expectAccess(path, expected)
	}

	expectSymlinkAccess := func(parent string, expected bool) {
		path := generateTempFilename(parent, "symlink")
		filesetup.CreateSymlink("/dev/null", path, os.Getuid())
		defer os.Remove(path)
		expectAccess(path, expected)

		path = generateTempFilename(parent, "symlink")
		filesetup.CreateSymlink("/dev", path, os.Getuid())
		defer os.Remove(path)
		expectAccess(filepath.Join(path, "null"), expected)
	}

	var blockedLocations = []string{
		"/mnt/stateful_partition",
		"/var",
	}

	var symlinkBlocked = []string{
		"/tmp",
	}

	var symlinkExceptions = []string{
		"/var/cache/echo",
		"/var/cache/vpd",
		"/var/lib/timezone",
		"/var/log",
	}

	// We also need to make sure that the restrictions apply to subdirectories, unless they are
	// treated specially (like the symlinkExceptions).
	for _, locs := range []*[]string{&blockedLocations, &symlinkBlocked, &symlinkExceptions} {
		for _, loc := range *locs {
			path, err := ioutil.TempDir(loc, "tast.security.StatefulPartitionHardening.")
			if err != nil {
				s.Fatal("Failed to create temp directory in ", loc)
			}
			defer os.RemoveAll(path)
			*locs = append(*locs, path)
		}
	}

	for _, loc := range blockedLocations {
		s.Log("Checking that symlinks and FIFOs are blocked in ", loc)
		expectSymlinkAccess(loc, false)
		expectFIFOAccess(loc, false)
	}

	for _, loc := range symlinkBlocked {
		s.Log("Checking that symlinks are blocked in ", loc)
		expectSymlinkAccess(loc, false)
	}

	for _, loc := range symlinkExceptions {
		s.Log("Checking that symlinks are allowed in ", loc)
		expectSymlinkAccess(loc, true)
	}
}
