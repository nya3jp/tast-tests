// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShareInvalidPaths,
		Desc:         "Tests that seneschal rejects paths that contain symlinks or point to non-regular files/directories",
		Contacts:     []string{"chirantan@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
	})
}

func ShareInvalidPaths(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)

	userData := filepath.Join("/home/user", pre.Container.VM.Concierge.GetOwnerID())
	downloads := filepath.Join(userData, "MyFiles/Downloads")
	sym := filepath.Join(downloads, "userdata")
	if err := os.Symlink(userData, sym); err != nil {
		s.Fatal("Failed to create symlink: ", err)
	}
	defer os.Remove(sym)

	if err := pre.Container.VM.ShareDownloadsPath(ctx, "userdata/Cookies", false); err == nil {
		s.Error("Successfully shared path containing symlink")
	}

	devs := []int{syscall.S_IFBLK, syscall.S_IFIFO, syscall.S_IFCHR, syscall.S_IFSOCK}
	devPath := filepath.Join(downloads, "dev_node")
	for _, dev := range devs {
		if err := syscall.Mknod(devPath, 0o600, dev); err != nil {
			s.Fatal("Failed to create dev node: ", err)
		}

		if err := pre.Container.VM.ShareDownloadsPath(ctx, "dev_node", false); err == nil {
			s.Error("Successfully shared path to non-regular file")
		}
		os.Remove(devPath)
	}
}
