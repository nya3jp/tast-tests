// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MountSymlink,
		Desc: "Verifies that the chromiumos LSM prevents paths with symlinks from being mounted",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"no_symlink_mount"},
		Attr:         []string{"group:mainline"},
	})
}

func MountSymlink(ctx context.Context, s *testing.State) {
	base, err := ioutil.TempDir("/tmp", "mount_symlink_test.")
	if err != nil {
		s.Fatal("Failed creating temp dir: ", err)
	}
	defer os.RemoveAll(base)

	mount := func(target string) error {
		s.Log("Mounting", target)
		cmd := testexec.CommandContext(ctx,
			"mount", "-c", "-n", "-t", "tmpfs", "-o", "nodev,noexec,nosuid", "test", target)
		err := cmd.Run()

		if err == nil {
			s.Log("Mount succeeded; unmounting")
			if err := testexec.CommandContext(ctx, "umount", "-n", target).Run(); err != nil {
				s.Errorf("Unmounting %v failed: %v", target, err)
			}
		} else {
			s.Log("Mount failed: ", err)
		}
		return err
	}

	mntTarget := filepath.Join(base, "mount_point")
	if err = os.Mkdir(mntTarget, 0700); err != nil {
		s.Fatal("Failed creating mount target: ", err)
	}
	if err = mount(mntTarget); err != nil {
		s.Errorf("Mounting %v failed: %v", mntTarget, err)
	}

	symTarget := filepath.Join(base, "symlink")
	if err = os.Symlink(filepath.Base(mntTarget), symTarget); err != nil {
		s.Fatal("Failed creating symlink: ", err)
	}
	if err = mount(symTarget); err == nil {
		s.Errorf("Mounting %v unexpectedly succeeded", symTarget)
	}
}
