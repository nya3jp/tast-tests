// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MiniContainer,
		Desc:         "Ensures Android mini container is upgraded after login",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func MiniContainer(ctx context.Context, s *testing.State) {
	// Make sure the Android container is stopped initially.
	upstart.StopJob(ctx, "ui")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := arc.BootstrapCommand(ctx, "/system/bin/true").Run(); err == nil {
			return errors.New("Android container still running")
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to wait for the Android container to stop: ", err)
	}

	initCh := make(chan error, 1)
	sleepCh := make(chan error, 1)

	// Start a goroutine that sends messages over channels as the Android mini container is brought up.
	go func() {
		// Wait for the Android mini container to start.
		initCh <- arc.WaitAndroidInit(ctx)

		// Start a process in the Android mini container. This process should be running
		// until we close *ARC.
		sleepCh <- arc.BootstrapCommand(ctx, "/system/bin/sleep", "86400").Run()
	}()

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Android mini container should be running at this point usually, but wait for it anyway.
	if err := <-initCh; err != nil {
		s.Fatal("Failed to wait for the Android mini container: ", err)
	}

	// Wait for a while after login to make sure the Android mini container is not turned down
	// even if we do not call arc.New immediately (crbug.com/872135).
	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		s.Fatal("Timed out while sleeping after login: ", ctx.Err())
	}

	select {
	case err := <-sleepCh:
		s.Fatal("Android mini container failed to upgrade: sleep process killed: ", err)
	default:
	}

	// Wait for Android to fully boot.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	select {
	case err := <-sleepCh:
		s.Fatal("Android mini container failed to upgrade: sleep process killed: ", err)
	default:
	}
}
