// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

type rebootConfig struct {
	// Number of repetitive reboots.
	numTrials int
	// Maximum time to wait for boot_completed flag to be set.
	bootTimeout time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Reboot,
		Desc:         "Checks whether Android can be repeatedly rebooted",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: rebootConfig{
				numTrials:   3,
				bootTimeout: 120 * time.Second,
			},
			ExtraSoftwareDeps: []string{"android"},
			Timeout:           10 * time.Minute,
			Pre:               arc.Booted(),
		}, {
			Name: "vm",
			Val: rebootConfig{
				numTrials:   3,
				bootTimeout: 120 * time.Second,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           10 * time.Minute,
			Pre:               arc.VMBooted(),
		}},
	})
}

func Reboot(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	numTrials := s.Param().(rebootConfig).numTrials
	for i := 0; i < numTrials; i++ {
		s.Logf("Trial %d/%d", i+1, numTrials)
		if err := runReboot(ctx, s, a); err != nil {
			s.Fatal("Failure: ", err)
		}
	}
}

// runReboot reboots Android and checks whether PID is changed after rebooting.
// It assumes that the connection to ADB has been already established at call-time.
func runReboot(ctx context.Context, s *testing.State, a *arc.ARC) error {
	oldPID, err := arc.InitPID()
	if err != nil {
		return errors.Wrap(err, "failed to get PID before reboot")
	}
	testing.ContextLog(ctx, "Rebooting Android via ADB")
	if err := a.Command(ctx, "reboot").Run(); err != nil {
		return errors.Wrap(err, "failed to reboot via ADB")
	}
	testing.ContextLog(ctx, "Re-establishing the connection to ADB")
	if err := arc.ConnectADB(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to ADB")
	}
	if err := waitBootCompleted(ctx, s, a); err != nil {
		return errors.Wrap(err, "reboot not completed before timeout")
	}
	newPID, err := arc.InitPID()
	if err != nil {
		return errors.Wrap(err, "failed to get PID after reboot")
	}
	if newPID == oldPID {
		return errors.New("PID did not change after reboot")
	}
	return nil
}

// waitBootCompleted waits for the flag ro.arc.boot_completed to be set.
// The flag is checked using getprop command via ADB.
func waitBootCompleted(ctx context.Context, s *testing.State, a *arc.ARC) error {
	const arcBootProp = "ro.arc.boot_completed"
	return testing.Poll(ctx, func(ctx context.Context) error {
		v, err := a.GetProp(ctx, arcBootProp)
		if err != nil {
			return errors.Wrapf(err, "failed to getprop %s", arcBootProp)
		}
		if v != "1" {
			return errors.New("reboot has not been completed yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: s.Param().(rebootConfig).bootTimeout})
}
