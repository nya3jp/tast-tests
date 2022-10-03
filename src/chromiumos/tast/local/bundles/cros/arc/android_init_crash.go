// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AndroidInitCrash,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test Android init crash handling",
		Contacts:     []string{"hashimoto@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 10 * time.Minute,
	})
}

func AndroidInitCrash(ctx context.Context, s *testing.State) {
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

	// Android init crash should result in Android reboot.
	s.Log("Inducing Android init crash")
	if err := unix.Kill(int(oldPID), unix.SIGTERM); err != nil {
		s.Fatal("Failed to kill Android init: ", err)
	}
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		pid, err := arc.InitPID()
		if err == nil && pid == oldPID {
			return errors.New("Old init still exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed to wait for old Android init to stop: ", err)
	}

	a.Close(ctx) // Ignore error and continue.
	a = nil

	// Make sure Android successfully boots.
	a, err = arc.New(ctx, s.OutDir())
	if err != nil {
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
