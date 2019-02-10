// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HardLinkRestrictions,
		Desc: "Verifies enforcement of hard link permissions",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
	})
}

func HardLinkRestrictions(ctx context.Context, s *testing.State) {
	// Check that hard link protection is enabled.
	// See https://wiki.ubuntu.com/SecurityTeam/Roadmap/KernelHardening for details.
	const procPath = "/proc/sys/fs/protected_hardlinks"
	if b, err := ioutil.ReadFile(procPath); err != nil {
		s.Fatalf("Failed to read %s: %v", procPath, err)
	} else if v := strings.TrimSpace(string(b)); v != "1" {
		s.Fatalf("%v contains %q; want \"1\"", procPath, v)
	}

	const user = "chronos" // arbitrary unprivileged user

	td, err := ioutil.TempDir("", "tast.security.HardLinkRestrictions.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)
	if err := os.Chmod(td, 0755); err != nil {
		s.Fatalf("Failed to set permissions on %v: %v", td, err)
	}

	// checkRead checks if user can or cannot read path, depending on expSuccess.
	checkRead := func(path string, expSuccess bool) {
		err := testexec.CommandContext(ctx, "sudo", "-u", user, "cat", path).Run()
		if expSuccess && err != nil {
			s.Errorf("%v wasn't able to read %v: %v", user, path, err)
		} else if !expSuccess && err == nil {
			s.Errorf("%v was able to read %v", user, path)
		}
	}

	// checkRead checks if user can or cannot write to path, depending on expSuccess.
	checkWrite := func(path string, expSuccess bool) {
		err := testexec.CommandContext(ctx, "sudo", "-u", user, "dd", "if=/etc/passwd", "of="+path).Run()
		if expSuccess && err != nil {
			s.Errorf("%v wasn't able to write %v: %v", user, path, err)
		} else if !expSuccess && err == nil {
			s.Errorf("%v was able to write %v", user, path)
		}
	}

	// checkLink checks if username can or cannot create a hard link from newname to oldname, depending on expSuccess.
	// If newname is created, it is removed before returning.
	checkLink := func(oldname, newname, username string, expSuccess bool) {
		err := testexec.CommandContext(ctx, "sudo", "-u", user, "ln", oldname, newname).Run()
		defer os.RemoveAll(newname)

		if expSuccess {
			if err != nil {
				s.Errorf("%v wasn't able to create %v -> %v: %v", user, newname, oldname, err)
			}
		} else if err == nil {
			s.Errorf("%v was able to create %v -> %v link", user, newname, oldname)
		} else if _, err = os.Stat(newname); err == nil {
			s.Errorf("%v created %v -> %v link despite ln failing", user, newname, oldname)
		}
	}

	s.Log("Checking hard links to regular files")

	// Verify that a hard link can't be created to a critical unwritable (and unreadable) target.
	checkLink("/etc/shadow", filepath.Join(td, "evil-hard-link"), user, false)

	// Create target files owned by root.
	secret := filepath.Join(td, "secret-target")
	filesetup.CreateFile(secret, "data", 0, 0600)
	readonly := filepath.Join(td, "world-readable-target")
	filesetup.CreateFile(readonly, "data", 0, 0444)
	writable := filepath.Join(td, "world-writable-target")
	filesetup.CreateFile(writable, "data", 0, 0666)

	// Check that read/write permissions are enforced.
	checkRead(secret, false)
	checkWrite(secret, false)
	checkRead(readonly, true)
	checkWrite(readonly, false)
	checkRead(writable, true)
	checkWrite(writable, true)

	// Create a directory owned by the user for holding links.
	ldir := filepath.Join(td, "links")
	uid, err := sysutil.GetUID(user)
	if err != nil {
		s.Fatal("Failed to find uid: ", err)
	}
	filesetup.CreateDir(ldir, int(uid), 0777)

	// Create a user-owned file.
	mine := filepath.Join(ldir, "mine")
	filesetup.CreateFile(mine, "", int(uid), 0644)

	// Permit creating links to self-owned or world-writable file.
	link := filepath.Join(ldir, "link")
	checkLink(mine, link, user, true)
	checkLink(writable, link, user, true)
	// Disallow links to unwritable files, though.
	checkLink(readonly, link, user, false)
	checkLink(secret, link, user, false)

	s.Log("Checking hard links to non-regular files")

	// Create a temp dir under /dev so we can test behavior around non-regular files.
	devdir, err := ioutil.TempDir("/dev", "tast.security.HardLinkRestrictions.")
	if err != nil {
		s.Fatal("Failed to create temp dir under /dev: ", err)
	}
	defer os.RemoveAll(devdir)
	if err := os.Chown(devdir, int(uid), 0); err != nil {
		s.Fatalf("Failed to chown %v to %v: %v", devdir, uid, err)
	}

	// Create a null device in the dir owned by root.
	null := filepath.Join(devdir, "null")
	if err := testexec.CommandContext(ctx, "mknod", "-m", "0666", null, "c", "1", "3").Run(); err != nil {
		s.Fatalf("Failed to create device node %v: %v", null, err)
	}

	// The user should be able to read and write from the device, but not create a hard link to it.
	checkRead(null, true)
	checkWrite(null, true)
	checkLink(null, filepath.Join(devdir, "link-to-unowned-device"), user, false)

	// Hard links should be permitted when created by the device owner.
	if err := os.Chown(null, int(uid), 0); err != nil {
		s.Fatalf("Failed to chown %v to %v: %v", devdir, uid, err)
	}
	devlink := filepath.Join(devdir, "link-to-owned-device")
	checkLink(null, devlink, user, true)

	// CAP_FOWNER should also grant the ability to create hard links to non-owned devices.
	checkLink(null, devlink, "root", true)
}
