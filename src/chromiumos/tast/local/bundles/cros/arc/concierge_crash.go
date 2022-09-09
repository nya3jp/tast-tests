// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConciergeCrash,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test concierge crash handling",
		Contacts:     []string{"hashimoto@chromium.org", "arcvm-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"}, // b/203428993
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      10 * time.Minute,
	})
}

func ConciergeCrash(ctx context.Context, s *testing.State) {
	// Shorten the context to save time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.UnRestrictARCCPU())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(cleanupCtx); err != nil {
			s.Fatal("Failed to close Chrome while (re)booting ARC: ", err)
		}
	}()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		// Note: |a| can be the arc.New() result above, or the one below after reboot.
		if a != nil {
			a.Close(ctx)
		}
	}()

	oldPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to get init PID before reboot: ", err)
	}

	// Concierge crash should result in Android reboot.
	s.Log("Inducing concierge crash")
	_, _, pid, err := upstart.JobStatus(ctx, "vm_concierge")
	if err != nil {
		s.Fatal("Failed to find vm_concierge job: ", err)
	}
	if err := testexec.CommandContext(ctx, "kill", fmt.Sprintf("%d", pid)).Run(); err != nil {
		s.Fatal("Failed to kill vm_concierge job: ", err)
	}

	s.Log("Waiting for old init process to exit")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		pid, err := arc.InitPID()
		if err != nil {
			return err
		}
		if pid == oldPID {
			return errors.New("init still exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed to wait for old init process to exit: ", err)
	}

	a.Close(ctx)
	a = nil

	// Make sure Android successfully boots.
	a, err = arc.New(ctx, s.OutDir())
	if err != nil {
		// We can assume a == nil at this point.
		s.Fatal("Failed to restart ARC: ", err)
	}
	// Additional check just to be sure.
	newPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to get init PID after reboot: ", err)
	}
	if newPID == oldPID {
		s.Fatal("Failure: init PID did not change")
	}
}
