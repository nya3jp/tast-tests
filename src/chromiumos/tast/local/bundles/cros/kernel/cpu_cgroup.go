// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

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
		Func:     CPUCgroup,
		Desc:     "Verifies that kernel CPU cgroups can be created",
		Contacts: []string{"mka@chromium.org", "chromeos-kernel-test@google.com"},
		Attr:     []string{"group:mainline"},
	})
}

func CPUCgroup(ctx context.Context, s *testing.State) {
	td, err := ioutil.TempDir("", "tast.kernel.CPUCgroup.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)

	if err := testexec.CommandContext(ctx, "mount", "-t", "cgroup", "cgroup",
		td, "-o", "cpu").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to mount cgroup controller: ", err)
	}
	defer testexec.CommandContext(ctx, "umount", td).Run(testexec.DumpLogOnError)

	// Use os.MkdirAll rather than os.Mkdir since the directory already exists
	// sometimes: https://crbug.com/958816
	dir := filepath.Join(td, "test")
	if err := os.MkdirAll(dir, 0777); err != nil {
		s.Fatal("Failed to create cgroup: ", err)
	}

	if fi, err := os.Stat(filepath.Join(dir, "tasks")); err != nil {
		s.Fatal("Tasks file is missing: ", err)
	} else if !fi.Mode().IsRegular() {
		s.Fatalf("%v is not a regular file (mode %v)", fi.Name(), fi.Mode())
	}
}
