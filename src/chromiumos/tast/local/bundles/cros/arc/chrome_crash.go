// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrash,
		Desc:         "Test chrome crash handling",
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

func ChromeCrash(ctx context.Context, s *testing.State) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := arc.InitPID()
		return err
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed to wait for Android init process: ", err)
	}
	oldPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to get init PID before reboot: ", err)
	}

	// Chrome crash should result in Android reboot.
	s.Log("Inducing chrome crash")

	chromePID, err := chrome.GetRootPID()
	if err != nil {
		s.Fatal("Failed to get chrome PID: ", err)
	}
	if err := syscall.Kill(chromePID, syscall.SIGSEGV); err != nil {
		s.Fatal("Failed to kill chrome: ", err)
	}

	s.Log("Waiting for Android init process to exit")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		pid, err := arc.InitPID()
		if err != nil && pid == oldPID {
			return errors.New("init still exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed to wait for Android init process to exit: ", err)
	}

	// Make sure Android rebooted.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := arc.InitPID()
		return err
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed to wait for restarted Android init process: ", err)
	}
}
