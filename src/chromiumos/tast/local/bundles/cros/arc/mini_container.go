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
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MiniContainer,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Ensures Android mini container is upgraded after login",
		Contacts: []string{
			"arc-core@google.com",
			"nya@chromium.org", // Tast port author.
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
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

	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to open syslog reader: ", err)
	}
	defer reader.Close()

	initCh := make(chan error, 1)
	sleepCh := make(chan error, 1)

	// Start a goroutine that sends messages over channels as the Android mini container is brought up.
	go func() {
		// Wait for the Android mini container to start.
		initCh <- arc.WaitAndroidInit(ctx, reader)

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
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Timed out while sleeping after login: ", err)
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
	defer a.Close(ctx)

	select {
	case err := <-sleepCh:
		s.Fatal("Android mini container failed to upgrade: sleep process killed: ", err)
	default:
	}
}
