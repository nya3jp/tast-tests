// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"os"
	"syscall"

	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemDirs,
		Desc: "Checks permissions of various system directories",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
	})
}

func SystemDirs(ctx context.Context, s *testing.State) {
	// If partitions have been remounted, there's no point in continuing.
	if ro, err := filesetup.ReadOnlyRootPartition(); err != nil {
		s.Fatal("Failed to check if root partition is mounted read-only: ", err)
	} else if !ro {
		s.Fatal("Root partition is mounted read/write; rootfs verification disabled?")
	}

	// Check that root-owned dirs have the expected ownership and permissions.
	for dir, mode := range map[string]os.FileMode{
		"/":                       0755,
		"/bin":                    0755,
		"/boot":                   0755,
		"/dev":                    0755,
		"/etc":                    0755,
		"/home":                   0755,
		"/lib":                    0755,
		"/media":                  0777,
		"/mnt":                    0755,
		"/mnt/stateful_partition": 0755,
		"/opt":                    0755,
		"/proc":                   0555,
		"/run":                    0755,
		"/sbin":                   0755,
		"/sys":                    0555, // 0555 since 3.4: https://crbug.com/213395
		"/tmp":                    0777,
		"/usr":                    0755,
		"/usr/bin":                0755,
		"/usr/lib":                0755,
		"/usr/local":              0755,
		"/usr/sbin":               0755,
		"/usr/share":              0755,
		"/var":                    0755,
		"/var/cache":              0755,
	} {
		fi, err := os.Stat(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				s.Errorf("Failed to stat %v: %v", dir, err)
			}
			continue
		}

		st := fi.Sys().(*syscall.Stat_t)
		if st.Uid != 0 || st.Gid != 0 {
			s.Errorf("%v is owned by %d:%d; want 0:0", dir, st.Uid, st.Gid)
		}
		if m := fi.Mode() & os.ModePerm; m != mode {
			s.Errorf("%v has mode %04o; want %04o", dir, m, mode)
		}
	}

	// tryWrite attempts to create and remove a file in dir.
	tryWrite := func(dir string) error {
		f, err := ioutil.TempFile(dir, ".tast.security.MountPerms.")
		if err != nil {
			return err
		}

		// Clean up the file.
		if err := f.Close(); err != nil {
			s.Error("Failed to close temp file: ", err)
		}
		if err := os.Remove(f.Name()); err != nil {
			s.Error("Failed to remove temp file: ", err)
		}
		return nil
	}

	// Check that root can't create files in read-only directories.
	for _, dir := range []string{
		"/",
		"/bin",
		"/boot",
		"/etc",
		"/lib",
		"/mnt",
		"/opt",
		"/sbin",
		"/usr",
		"/usr/bin",
		"/usr/lib",
		"/usr/sbin",
		"/usr/share",
	} {
		if err := tryWrite(dir); err == nil {
			s.Error("Unexpectedly able to create file in read-only dir ", dir)
		}
	}

	// Check that root can create files in read/write dirs.
	for _, dir := range []string{
		"/run",
		"/usr/local",
		"/var",
		"/var/cache",
		"/var/lib",
		"/var/log",
	} {
		if err := tryWrite(dir); err != nil {
			s.Errorf("Failed to create file in read/write dir %v: %v", dir, err)
		}
	}
}
