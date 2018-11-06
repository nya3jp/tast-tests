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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SymlinkRestrictions,
		Desc: "Verifies that unsafe symlinks are blocked",
		Attr: []string{"informational"},
	})
}

func SymlinkRestrictions(ctx context.Context, s *testing.State) {
	// Check that symlink protection is enabled.
	// See https://wiki.ubuntu.com/SecurityTeam/Roadmap/KernelHardening for details.
	const procPath = "/proc/sys/fs/protected_symlinks"
	if b, err := ioutil.ReadFile(procPath); err != nil {
		s.Fatalf("Failed to read %s: %v", procPath, err)
	} else if v := strings.TrimSpace(string(b)); v != "1" {
		s.Fatalf("%v contains %q instead of \"1\"", procPath, v)
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
	filesetup.CreateSymlink(s, "/etc/shadow", linkPath, filesetup.GetUID(s, "chronos"))
	if _, err := ioutil.ReadFile(linkPath); err == nil {
		s.Errorf("Following malicious symlink %v was permitted", linkPath)
	}

	// readAsUser reads path as username. If expSuccess is true, its contents are compared against expData.
	// Otherwise, an error is reported only if the read was successful.
	readAsUser := func(path, username string, expSuccess bool, expData string) {
		b, err := testexec.CommandContext(ctx, "su", "-c", "cat "+testexec.ShellEscape(path), username).Output()
		if expSuccess {
			if err != nil {
				s.Errorf("Failed to read %v as %v: %v", path, username, err)
			} else if string(b) != expData {
				s.Errorf("Read %v as %v and got %q; want %q", path, username, string(b), expData)
			}
		} else {
			if err == nil {
				s.Errorf("Unexpectedly able to read %v as %v", path, username)
			}
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
		uid := filesetup.GetUID(s, username)
		switch st {
		case fileOwnedByUser:
			filesetup.CreateFile(s, checkPath, "initial contents", uid, 0644)
		case fileMissing:
			if err := os.RemoveAll(checkPath); err != nil {
				s.Fatalf("Failed to remove %v: %v", checkPath, err)
			}
		}

		err := testexec.CommandContext(ctx, "su", "-c", "dd if=/etc/passwd of="+testexec.ShellEscape(writePath), username).Run()
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
		} else if u := int(fi.Sys().(*syscall.Stat_t).Uid); u != uid {
			s.Errorf("%v owned by %v after writing to %v as %v; want %v", checkPath, u, writePath, username, uid)
		}
	}

	for _, tc := range []struct {
		user1  string // directory owner
		user2  string // other user
		sticky bool   // whether dir should be sticky
	}{
		{"root", "chronos", false},
		{"chronos", "root", false},
		{"root", "chronos", true},
		{"chronos", "root", true},
	} {
		uid1 := filesetup.GetUID(s, tc.user1)
		uid2 := filesetup.GetUID(s, tc.user2)

		var mode os.FileMode = 0777
		dirType := "regular world-writable dir"
		if tc.sticky {
			mode |= os.ModeSticky
			dirType = "sticky world-writable dir"
		}
		s.Logf("Using a %v owned by %v", dirType, tc.user1)

		// Create a world-writable directory owned by the first user.
		dir, err := ioutil.TempDir(td, fmt.Sprintf("%v-%v-sticky=%v.", tc.user1, tc.user2, tc.sticky))
		if err != nil {
			s.Fatal("Failed creating temp dir: ", err)
		}
		if err := os.Chown(dir, uid1, 0); err != nil {
			s.Fatalf("Failed to chown %v to %v: %v", dir, uid1, err)
		}
		if err := os.Chmod(dir, mode); err != nil {
			s.Fatalf("Failed to chown %v to %v: %v", dir, mode, err)
		}

		// Verify basic stickiness behavior: try to remove a file owned by the directory owner as the other user.
		toDelete := filepath.Join(dir, "remove.me")
		filesetup.CreateFile(s, toDelete, "I can be deleted in a non-sticky directory", uid1, 0644)
		err = testexec.CommandContext(ctx, "su", "-c", "rm -f "+testexec.ShellEscape(toDelete), tc.user2).Run()
		wantErr := tc.sticky && uid2 != 0 // should be able to delete unless running in sticky dir as non-root
		if err == nil && wantErr {
			s.Errorf("%v was able to delete file owned by %v in %s", tc.user2, tc.user1, dirType)
		} else if err != nil && !wantErr {
			s.Errorf("%v wasn't able to delete file owned by %v in %s", tc.user2, tc.user1, dirType)
		}

		// Create target files.
		publicTarget := filepath.Join(dir, "target")
		const publicData = "not secret"
		filesetup.CreateFile(s, publicTarget, publicData, 0, 0644)

		targetUser1 := filepath.Join(dir, "target-owned-by-"+tc.user1)
		dataUser1 := "secret owned by " + tc.user1
		filesetup.CreateFile(s, targetUser1, dataUser1, uid1, 0400)

		targetUser2 := filepath.Join(dir, "target-owned-by-"+tc.user2)
		dataUser2 := "secret owned by " + tc.user2
		filesetup.CreateFile(s, targetUser2, dataUser2, uid2, 0400)

		// Create symlinks owned by both users that point at the public target.
		symlinkUser1 := filepath.Join(dir, "symlink-owned-by-"+tc.user1)
		filesetup.CreateSymlink(s, publicTarget, symlinkUser1, uid1)
		symlinkUser2 := filepath.Join(dir, "symlink-owned-by-"+tc.user2)
		filesetup.CreateSymlink(s, publicTarget, symlinkUser2, uid2)

		// The public target should be directly readable by both users.
		readAsUser(publicTarget, tc.user1, true, publicData)
		readAsUser(publicTarget, tc.user2, true, publicData)

		// Non-public targets should only be readable by their owners.
		// UID 0 should also be able to read other users' files due to DAC_OVERRIDE.
		readAsUser(targetUser1, tc.user1, true, dataUser1)
		readAsUser(targetUser2, tc.user2, true, dataUser2)
		readAsUser(targetUser1, tc.user2, uid2 == 0, dataUser1)
		readAsUser(targetUser2, tc.user1, uid1 == 0, dataUser2)

		// The public target should be readable via a symlink by the symlink's owner.
		readAsUser(symlinkUser1, tc.user1, true, publicData)
		readAsUser(symlinkUser2, tc.user2, true, publicData)
		// It should also be universally readable through a symlink owned by the directory owner (i.e. tc.user1).
		readAsUser(symlinkUser1, tc.user2, true, publicData)
		// When the symlink owner doesn't match a sticky directory owner, reading as another user should be blocked.
		readAsUser(symlinkUser2, tc.user1, !tc.sticky, publicData)

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
