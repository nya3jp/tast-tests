// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBGuard,
		Desc:         "Check that USBGuard related feature flags work as intended",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"usbguard"},
	})
}

func USBGuard(ctx context.Context, s *testing.State) {
	const (
		usbguardFeature    = "USBGuard"
		usbbouncerFeature  = "USBBouncer"
		usbguardJob        = "usbguard"
		usbguardWrapperJob = "usbguard-wrapper"
		usbguardProcess    = "usbguard-daemon"
		usbguardPolicy     = "/run/usbguard/rules.conf"

		screenLockedEvent   = "screen-locked"
		screenUnlockedEvent = "screen-unlocked"

		defaultTimeout = 10 * time.Second
	)

	checkEnabled := func(feature string, expected bool) error {
		const (
			dbusName      = "org.chromium.ChromeFeaturesService"
			dbusPath      = "/org/chromium/ChromeFeaturesService"
			dbusInterface = "org.chromium.ChromeFeaturesServiceInterface"
			dbusMethod    = ".IsFeatureEnabled"
		)

		_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
		if err != nil {
			return err
		}

		enabled := false
		if err := obj.CallWithContext(ctx, dbusInterface+dbusMethod, 0, feature).Store(&enabled); err != nil {
			return err
		}

		if enabled != expected {
			return errors.Errorf("Got unexpected feature state of %q! (%t != %t)", feature, expected, enabled)
		}
		return nil
	}

	expectUsbguardRunning := func(running bool, onLockScreen bool) error {
		var (
			goal  = upstart.StopGoal
			state = upstart.WaitingState
		)
		if running {
			goal = upstart.StartGoal
			state = upstart.RunningState
		}
		err := upstart.WaitForJobStatus(ctx, usbguardJob, goal, state, upstart.TolerateWrongGoal, defaultTimeout)
		if err != nil {
			return errors.Errorf("Unexpected job status for %q: %v", usbguardJob, err)
		}

		if running {
			_, err = os.Stat(usbguardPolicy)
			if err != nil {
				return errors.Errorf("Cannot find usbguard policy when it should exist. %v", err)
			}
		} else if !onLockScreen {
			err := upstart.WaitForJobStatus(ctx, usbguardWrapperJob, goal, state, upstart.TolerateWrongGoal, defaultTimeout)
			if err != nil {
				return errors.Errorf("Failed to wait on wrapper job to stop. %v", err)
			}
			if _, err = os.Stat(usbguardPolicy); !os.IsNotExist(err) {
				return errors.Errorf("Unexpected error checking for policy file: %v", err)
			}
		}
		return nil
	}

	singleUSBGuardTest := func(usbguardEnabled bool, usbbouncerEnabled bool) {
		boolToEnable := func(value bool) string {
			if value {
				return "enable"
			}
			return "disable"
		}
		cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{
			"--" + boolToEnable(usbguardEnabled) + "-features=" + usbguardFeature,
			"--" + boolToEnable(usbbouncerEnabled) + "-features=" + usbbouncerFeature}))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		if checkEnabled(usbguardFeature, usbguardEnabled) != nil ||
			checkEnabled(usbbouncerFeature, usbbouncerEnabled) != nil {
			s.Error(err)
			return
		}

		err = expectUsbguardRunning(false, false)
		if err != nil {
			s.Errorf("Unexpected initial job status for %q: %v", usbguardJob, err)
		}

		s.Logf("Sending %q event", screenLockedEvent)
		upstart.EmitEvent(ctx, screenLockedEvent)
		err = expectUsbguardRunning(usbguardEnabled, true)
		if err != nil {
			s.Error(err)
		}

		if usbguardEnabled {
			// Make sure the daemon respawns.
			s.Logf("Killing %q to check for respawn", usbguardProcess)
			err := testing.Poll(ctx, func(ctx context.Context) error {
				c := testexec.CommandContext(ctx, "killall", usbguardProcess)
				_, err := c.Output()
				if err != nil {
					c.DumpLog(ctx)
					err = errors.Errorf("Failed to kill %q: %v", usbguardProcess, err)
				}
				return err
			}, &testing.PollOptions{Timeout: defaultTimeout})
			if err != nil {
				s.Error(err)
			}
			err = expectUsbguardRunning(true, true)
			if err != nil {
				s.Errorf("Unexpected job status for %q: %v", usbguardJob, err)
			}
		}

		s.Logf("Sending %q event", screenUnlockedEvent)
		upstart.EmitEvent(ctx, screenUnlockedEvent)
		err = expectUsbguardRunning(false, false)
		if err != nil {
			s.Errorf("Unexpected final job status for %q: %v", usbguardJob, err)
		}
	}

	singleUSBGuardTest(true, false)
	singleUSBGuardTest(false, false)
	// Testing USB Bouncer requires the usb_bouncer ebuild which isn't installed by default yet.
}
