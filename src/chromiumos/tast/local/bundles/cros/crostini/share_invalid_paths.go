// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShareInvalidPaths,
		Desc:         "Tests that seneschal rejects paths that contain symlinks or point to non-regular files/directories",
		Contacts:     []string{"chirantan@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params:       crostini.MakeTestParams(crostini.TestInformational),
	})
}

func ShareInvalidPaths(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx, pre.Container)

	userData := filepath.Join("/home/user", pre.Container.VM.Concierge.GetOwnerID())
	downloads := filepath.Join(userData, "MyFiles/Downloads")
	sym := filepath.Join(downloads, "userdata")
	if err := os.Symlink(userData, sym); err != nil {
		s.Fatal("Failed to create symlink: ", err)
	}
	defer os.Remove(sym)

	if path, err := pre.Container.VM.ShareDownloadsPath(ctx, "userdata/Cookies", false); err == nil {
		s.Error("Unexpectedly shared path containing symlink")
		if err := pre.Container.VM.UnshareDownloadsPath(ctx, path); err != nil {
			s.Fatal("Failed to un-share path containing symlink: ", err)
		}
	} else if !strings.Contains(err.Error(), "symlink") {
		s.Error("Unexpected error when sharing a path containing a symlink: ", err)
	}

	devs := []int{syscall.S_IFBLK, syscall.S_IFIFO, syscall.S_IFCHR, syscall.S_IFSOCK}
	for _, dev := range devs {
		p := path.Join(downloads, fmt.Sprintf("dev_node%d", dev))
		if err := syscall.Mknod(p, 0o600, dev); err != nil {
			s.Fatal("Failed to create dev node: ", err)
		}
		defer os.Remove(p)

		if sharedPath, err := pre.Container.VM.ShareDownloadsPath(ctx, path.Base(p), false); err == nil {
			s.Error("Unexpectedly shared path to non-regular file")
			if err := pre.Container.VM.UnshareDownloadsPath(ctx, sharedPath); err != nil {
				s.Fatal("Failed to un-share device node: ", err)
			}
		} else if !strings.Contains(err.Error(), "non-regular") {
			s.Error("Unexpected error when sharing a path to a non-regular file: ", err)
		}
	}
}
