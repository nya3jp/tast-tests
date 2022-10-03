// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MojoProxyCrash,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test mojo proxy crash handling",
		Contacts:     []string{"hashimoto@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      10 * time.Minute,
	})
}

func MojoProxyCrash(ctx context.Context, s *testing.State) {
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
		if a != nil {
			a.Close(ctx)
		}
	}()

	oldPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to get init PID before reboot: ", err)
	}

	// Forceful kill of the mojo proxy.
	s.Log("Inducing MojoProxy crash")
	const mojoProxyPath = "/usr/bin/arcvm_server_proxy"
	if proc, err := procutil.FindUnique(procutil.ByExe(mojoProxyPath)); err != nil {
		s.Fatal("Failed to find MojoProxy process: ", err)
	} else if err := proc.SendSignal(unix.SIGABRT); err != nil {
		s.Fatal("Failed to kill MojoProxy: ", err)
	}

	s.Log("Waiting for old init process to exit")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		r, err := arc.InitExists()
		if err != nil {
			return err
		}
		if r {
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
