// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Tool,
		Desc: "Check that the hps debugging tool can interact with HPS",
		Contacts: []string{
			"dcallagh@chromium.org", // Test author
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"hps"},
		HardwareDeps: hwdep.D(hwdep.HPS()),
	})
}

func Tool(ctx context.Context, s *testing.State) {
	// Stop hpsd and unbind the HPS kernel driver, so that we can interact
	// with HPS directly.
	const job = "hpsd"
	s.Logf("Stopping %s", job)
	if err := upstart.StopJob(ctx, job); err != nil {
		s.Fatalf("Failed to stop %s: %v", job, err)
	}
	s.Log("Unbinding HPS kernel driver")
	cmd := testexec.CommandContext(ctx, "hps", "unbind")
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to unbind HPS kernel driver: ", err)
	}
	// Re-bind the HPS kernel driver at the end of the test.
	// Note that this will also trigger hpsd to be started again.
	defer func() {
		s.Log("Re-binding HPS kernel driver")
		cmd := testexec.CommandContext(ctx, "hps", "bind")
		if err := cmd.Run(); err != nil {
			s.Error("Failed to re-bind HPS kernel driver: ", err)
		}
	}()

	// Check `hps status` while in stage0.
	cmd = testexec.CommandContext(ctx, "hps", "status")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to read status in stage0: ", err)
	}
	pattern := regexp.MustCompile(`(?m)^Register   2: 0x.... \(kSysStatus\) kOK\|kStage0`)
	if !pattern.MatchString(string(output)) {
		s.Fatal("`hps status` does not show status register in stage0")
	}

	// Launch stage1.
	s.Log("Launching stage1")
	cmd = testexec.CommandContext(ctx, "hps", "cmd", "launch")
	if err := cmd.Run(); err != nil {
		s.Error("Failed to launch stage1: ", err)
	}

	// HPS stops responding briefly while it launches stage1.
	// Wait for it to start answering again.
	s.Log("Waiting for HPS after launching stage1")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := testexec.CommandContext(ctx, "hps", "status").Run(); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{
		Interval: 10 * time.Millisecond,
		Timeout:  250 * time.Millisecond,
	}); err != nil {
		s.Fatal("Timed out waiting for HPS to respond after launching stage1")
	}

	// Check `hps status` while in stage1.
	cmd = testexec.CommandContext(ctx, "hps", "status")
	output, err = cmd.Output()
	if err != nil {
		s.Fatal("Failed to read status in stage1: ", err)
	}
	pattern = regexp.MustCompile(`(?m)^Register   2: 0x.... \(kSysStatus\) kOK\|kStage1`)
	if !pattern.MatchString(string(output)) {
		s.Fatal("`hps status` does not show status register in stage1")
	}
}
