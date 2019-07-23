// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/crash"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SandboxLinuxUnittests,
		Desc: "Runs the sandbox_linux_unittests Chrome binary",
		Attr: []string{"informational"},
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
	})
}

func SandboxLinuxUnittests(ctx context.Context, s *testing.State) {
	const exec = "sandbox_linux_unittests"

	// This test causes intentional crashes. Clean up after it.
	defer func() {
		crashes, err := crash.GetCrashes(crash.DefaultDirs()...)
		if err != nil {
			s.Error("Failed to get crash files: ", err)
			return
		}
		s.Log("Deleting (expected) crash file(s) for ", exec)
		for _, p := range crashes {
			if fn := filepath.Base(p); !strings.HasPrefix(fn, exec+".") {
				continue
			}
			if err := os.Remove(p); err != nil {
				s.Errorf("Failed to delete %v: %v", p, err)
			}
		}
	}()

	if ts, err := bintest.Run(ctx, exec, nil, s.OutDir()); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}
}
