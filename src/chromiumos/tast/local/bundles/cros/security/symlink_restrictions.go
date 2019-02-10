// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SymlinkRestrictions,
		Desc: "Verifies that unsafe symlinks are blocked",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
	})
}

func SymlinkRestrictions(ctx context.Context, s *testing.State) {
	// Check that symlink protection is enabled.
	// See https://wiki.ubuntu.com/SecurityTeam/Roadmap/KernelHardening for details.
	const procPath = "/proc/sys/fs/protected_symlinks"
	if b, err := ioutil.ReadFile(procPath); err != nil {
		s.Fatalf("Failed to read %s: %v", procPath, err)
	} else if v := strings.TrimSpace(string(b)); v != "1" {
		s.Fatalf("%v contains %q; want \"1\"", procPath, v)
	}
	chronosUID, err := sysutil.GetUID("chronos")
	if err != nil {
		s.Fatal("Failed to find uid: ", err)
	}

	td, err := ioutil.TempDir("", "tast.security.SymlinkRestrictions.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)
	if err := os.Chmod(td, 0777|os.ModeSticky); err != nil {
		s.Fatalf("Failed to set permissions on %v: %v", td, err)
	}

	// As an initial high-level check, verify that we won't follow a chronos-owned symlink to a restricted file.
	linkPath := filepath.Join(td, "evil-symlink")
	filesetup.CreateSymlink("/etc/shadow", linkPath, int(chronosUID))
	if _, err := ioutil.ReadFile(linkPath); err == nil {
		s.Errorf("Following malicious symlink %v was permitted", linkPath)
	}

	// readAsUser reads path as username. If expSuccess is true, an error is reported if the read fails.
	// Otherwise, an error is reported only if the read was successful.
	readAsUser := func(path, username string, expSuccess bool) {
		err := testexec.CommandContext(ctx, "sudo", "-u", username, "cat", path).Run()
		if expSuccess && err != nil {
			s.Errorf("Failed to read %v as %v: %v", path, username, err)
		} else if !expSuccess && err == nil {
			s.Errorf("Unexpectedly able to read %v as %v", path, username)
		}
	}

	// initialState describes an initial file state for writeAsUser.
	type initialState int
	const (
		// fileOwnedByUser indicates that the file should already exist and be owned by the supplied user.
		fileOwnedByUser initialState = iota
		// fileMissing indicates that the file should initially not be present.
		fileMissing
	)

	// writeAsUser attempts to write arbitrary data to writePath as username.
	// initialState describes checkPath's initial state.
	// If expSuccess is true, checkPath is verified to exist and to be owned by username.
	// Otherwise, an error is reported only if the write is successful.
	writeAsUser := func(writePath, checkPath, username string, st initialState, expSuccess bool) {
		uid, err := sysutil.GetUID(username)
		if err != nil {
			s.Fatal("Failed to find uid: ", err)
		}
		switch st {
		case fileOwnedByUser:
			filesetup.CreateFile(checkPath, "initial contents", int(uid), 0644)
		case fileMissing:
			if err := os.RemoveAll(checkPath); err != nil {
				s.Fatalf("Failed to remove %v: %v", checkPath, err)
			}
		}

		err = testexec.CommandContext(ctx, "sudo", "-u", username, "dd", "if=/etc/passwd", "of="+writePath).Run()
		if !expSuccess {
			if err == nil {
				s.Errorf("Writing to %v as %v unexpectedly succeeded", writePath, username)
			}
			return
		}
		if err != nil {
			s.Errorf("Writing to %v as %v failed: %v", writePath, username, err)
			return
		}

		if fi, err := os.Stat(checkPath); os.IsNotExist(err) {
			s.Errorf("%v doesn't exist after writing to %v as %v", checkPath, writePath, username)
		} else if err != nil {
			s.Errorf("Failed to stat %v after writing to %v as %v: %v", checkPath, writePath, username, err)
		} else if u := fi.Sys().(*syscall.Stat_t).Uid; u != uid {
			s.Errorf("%v owned by %v after writing to %v as %v; want %v", checkPath, u, writePath, username, uid)
		}
	}

	for _, tc := range []struct {
		user1  string // directory owner
		user2  string // other user
		uid1   uint32
		uid2   uint32
		sticky bool // whether dir should be sticky
	}{
		{"root", "chronos", 0, chronosUID, false},
		{"chronos", "root", chronosUID, 0, false},
		{"root", "chronos", 0, chronosUID, true},
		{"chronos", "root", chronosUID, 0, true},
	} {
		var mode os.FileMode = 0777
		dirType := "regular world-writable dir"
		if tc.sticky {
			mode |= os.ModeSticky
			dirType = "sticky world-writable dir"
		}
		s.Logf("Using a %v owned by %v", dirType, tc.user1)

		// Create a world-writable directory owned by the first user.
		dir := filepath.Join(td, fmt.Sprintf("%v-%v-sticky=%v", tc.user1, tc.user2, tc.sticky))
		filesetup.CreateDir(dir, int(tc.uid1), mode)

		// Verify basic stickiness behavior: try to remove a file owned by the directory owner as the other user.
		toDelete := filepath.Join(dir, "remove.me")
		filesetup.CreateFile(toDelete, "I can be deleted in a non-sticky directory", int(tc.uid1), 0644)
		err = testexec.CommandContext(ctx, "sudo", "-u", tc.user2, "rm", "-f", toDelete).Run()
		wantErr := tc.sticky && tc.uid2 != 0 // should be able to delete unless running in sticky dir as non-root
		if err == nil && wantErr {
			s.Errorf("%v was able to delete file owned by %v in %s", tc.user2, tc.user1, dirType)
		} else if err != nil && !wantErr {
			s.Errorf("%v wasn't able to delete file owned by %v in %s", tc.user2, tc.user1, dirType)
		}

		// Create target files.
		publicTarget := filepath.Join(dir, "target")
		filesetup.CreateFile(publicTarget, "not secret", 0, 0644)

		targetUser1 := filepath.Join(dir, "target-owned-by-"+tc.user1)
		filesetup.CreateFile(targetUser1, "secret owned by "+tc.user1, int(tc.uid1), 0400)

		targetUser2 := filepath.Join(dir, "target-owned-by-"+tc.user2)
		filesetup.CreateFile(targetUser2, "secret owned by "+tc.user2, int(tc.uid2), 0400)

		// Create symlinks owned by both users that point at the public target.
		symlinkUser1 := filepath.Join(dir, "symlink-owned-by-"+tc.user1)
		filesetup.CreateSymlink(publicTarget, symlinkUser1, int(tc.uid1))
		symlinkUser2 := filepath.Join(dir, "symlink-owned-by-"+tc.user2)
		filesetup.CreateSymlink(publicTarget, symlinkUser2, int(tc.uid2))

		// The public target should be directly readable by both users.
		readAsUser(publicTarget, tc.user1, true)
		readAsUser(publicTarget, tc.user2, true)

		// Non-public targets should only be readable by their owners.
		// UID 0 should also be able to read other users' files due to DAC_OVERRIDE.
		readAsUser(targetUser1, tc.user1, true)
		readAsUser(targetUser2, tc.user2, true)
		readAsUser(targetUser1, tc.user2, tc.uid2 == 0)
		readAsUser(targetUser2, tc.user1, tc.uid1 == 0)

		// The public target should be readable via a symlink by the symlink's owner.
		readAsUser(symlinkUser1, tc.user1, true)
		readAsUser(symlinkUser2, tc.user2, true)
		// It should also be universally readable through a symlink owned by the directory owner (i.e. tc.user1).
		readAsUser(symlinkUser1, tc.user2, true)
		// When the symlink owner doesn't match a sticky directory owner, reading as another user should be blocked.
		readAsUser(symlinkUser2, tc.user1, !tc.sticky)

		// A self-owned file should be directly writable by both users.
		writeAsUser(publicTarget, publicTarget, tc.user1, fileOwnedByUser, true)
		writeAsUser(publicTarget, publicTarget, tc.user2, fileOwnedByUser, true)
		// It should also be writable via self-owned symlinks.
		writeAsUser(publicTarget, publicTarget, tc.user1, fileOwnedByUser, true)
		writeAsUser(publicTarget, publicTarget, tc.user2, fileOwnedByUser, true)
		// The second user should be able to use the directory owner's symlink to write the file.
		writeAsUser(symlinkUser1, publicTarget, tc.user2, fileOwnedByUser, true)
		// Non-directory-owner symlinks shouldn't be writable when the dir is sticky.
		writeAsUser(symlinkUser2, publicTarget, tc.user1, fileOwnedByUser, !tc.sticky)

		// A file should be directly creatable by both users.
		writeAsUser(publicTarget, publicTarget, tc.user1, fileMissing, true)
		writeAsUser(publicTarget, publicTarget, tc.user2, fileMissing, true)
		// The file should also be creatable via a self-owned symlink.
		writeAsUser(symlinkUser1, publicTarget, tc.user1, fileMissing, true)
		writeAsUser(symlinkUser2, publicTarget, tc.user2, fileMissing, true)
		// The second user should be able to use the directory owner's symlink to create the file.
		writeAsUser(symlinkUser1, publicTarget, tc.user2, fileMissing, true)
		// Creating a file with a non-directory-owner symlink shouldn't work when the dir is sticky.
		writeAsUser(symlinkUser2, publicTarget, tc.user1, fileMissing, !tc.sticky)
	}
}
