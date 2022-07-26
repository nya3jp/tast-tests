// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     FlashromTester,
		Desc:     "Tast wrapper that runs flashrom_tester",
		Contacts: []string{"nartemiev@google.com", "cros-flashrom-team@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  30 * time.Minute,
		Params: []testing.Param{
			{
				Fixture: fixture.DevModeGBB,
			},
		},
	})
}

func FlashromTester(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	cmd := h.DUT.Conn().CommandContext(ctx, "/bin/bash", "-c", "echo stdout; echo stderr 1>&2")

	// this kind of exec works:
	// cmd := testexec.CommandContext(ctx, "/bin/bash", "-c", "echo stdout; echo stderr 1>&2")

	stderrFile, err := os.Create(filepath.Join(s.OutDir(), "flashrom_tester_stderr.txt"))
	if err != nil {
		s.Fatal("os.Open failed: ", err)
	}
	cmd.Stderr = stderrFile

	s.Log("Starting flashrom_tester")
	if err := cmd.Start(); err != nil {
		s.Fatal("Start() failed: ", err)
	}

	closer := func() {
		if err := stderrFile.Close(); err != nil {
			s.Error("Failed to close stderr: ", err)
		}
	}

	if true {
		closer()
	} else {
		// deferring the close works, even though we already forked
		defer closer()
	}

	defer func() {
		if err := cmd.Wait(); err != nil {
			s.Error("flashrom_tester failed: ", err)
		}
	}()
}
