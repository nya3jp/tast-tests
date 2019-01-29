// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"strings"
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
		Desc:         "Check that USBGuard-related feature flags work as intended",
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

		jobTimeout     = 10 * time.Second
		respawnTimeout = 10 * time.Second
	)

	isFeatureEnabled := func(feature string) (bool, error) {
		const (
			dbusName      = "org.chromium.ChromeFeaturesService"
			dbusPath      = "/org/chromium/ChromeFeaturesService"
			dbusInterface = "org.chromium.ChromeFeaturesServiceInterface"
			dbusMethod    = ".IsFeatureEnabled"
		)
		enabled := false

		_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
		if err != nil {
			return enabled, err
		}

		if err := obj.CallWithContext(ctx, dbusInterface+dbusMethod, 0, feature).Store(&enabled); err != nil {
			return enabled, err
		}

		return enabled, nil
	}

	expectUsbguardRunning := func(running, onLockScreen bool) error {
		goal := upstart.StopGoal
		state := upstart.WaitingState
		if running {
			goal = upstart.StartGoal
			state = upstart.RunningState
		}
		err := upstart.WaitForJobStatus(ctx, usbguardJob, goal, state, upstart.TolerateWrongGoal, jobTimeout)
		if err != nil {
			return errors.Wrapf(err, "unexpected job status for %v", usbguardJob)
		}

		if running {
			_, err = os.Stat(usbguardPolicy)
			if err != nil {
				return errors.Wrapf(err, "failed finding policy %v", usbguardPolicy)
			}
		} else if !onLockScreen {
			err := upstart.WaitForJobStatus(ctx, usbguardWrapperJob, goal, state, upstart.TolerateWrongGoal, jobTimeout)
			if err != nil {
				return errors.Wrapf(err, "failed to wait on job %v to stop", usbguardWrapperJob)
			}
			if _, err = os.Stat(usbguardPolicy); err == nil {
				return errors.Errorf("policy %v unexpectedly exists", usbguardPolicy)
			} else if err != nil && !os.IsNotExist(err) {
				return errors.Wrapf(err, "failed checking policy %v", usbguardPolicy)
			}
		}
		return nil
	}

	checkUsbguardRespawn := func() error {
		err := testing.Poll(ctx, func(ctx context.Context) error {
			c := testexec.CommandContext(ctx, "killall", usbguardProcess)
			_, err := c.Output()
			if err != nil {
				c.DumpLog(ctx)
				err = errors.Wrapf(err, "failed to kill %v", usbguardProcess)
			}
			return err
		}, &testing.PollOptions{Timeout: respawnTimeout})
		if err != nil {
			return err
		}

		if err = expectUsbguardRunning(true, true); err != nil {
			return errors.Wrapf(err, "failed to check that %v job was respawned", usbguardJob)
		}
		return nil
	}

	runTest := func(usbguardEnabled, usbbouncerEnabled bool) {
		featureValues := map[string]bool{
			usbguardFeature:   usbguardEnabled,
			usbbouncerFeature: usbbouncerEnabled,
		}
		var enabled, disabled []string
		for name, val := range featureValues {
			if val {
				enabled = append(enabled, name)
			} else {
				disabled = append(disabled, name)
			}
		}

		var args []string
		if len(enabled) > 0 {
			args = append(args, "--enable-features="+strings.Join(enabled, ","))
		}
		if len(disabled) > 0 {
			args = append(args, "--disable-features="+strings.Join(disabled, ","))
		}

		cr, err := chrome.New(ctx, chrome.ExtraArgs(args))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		boolToEnabled := func(value bool) string {
			if value {
				return "enabled"
			}
			return "disabled"
		}

		for name, val := range featureValues {
			if en, err := isFeatureEnabled(name); err != nil {
				s.Error("Failed if the %v feature is enabled: ", err)
				return
			} else if en != val {
				s.Errorf("The %v feature is supposed to be %s but it is %s instead", name,
					boolToEnabled(val), boolToEnabled(en))
				return
			}
		}

		if err = expectUsbguardRunning(false, false); err != nil {
			s.Errorf("Unexpected initial job status for %q: %v", usbguardJob, err)
			return
		}

		s.Logf("Sending %q event", screenLockedEvent)
		upstart.EmitEvent(ctx, screenLockedEvent)
		if err = expectUsbguardRunning(usbguardEnabled, true); err != nil {
			s.Error("Failed to check if usbguard is running: ", err)
		}

		if usbguardEnabled {
			s.Logf("Killing %q to check for respawn", usbguardProcess)
			if err = checkUsbguardRespawn(); err != nil {
				s.Errorf("Failed to check that %v job respawns: %v", usbguardJob, err)
			}
		}

		s.Logf("Sending %q event", screenUnlockedEvent)
		upstart.EmitEvent(ctx, screenUnlockedEvent)
		if err = expectUsbguardRunning(false, false); err != nil {
			s.Errorf("Unexpected final job status for %q: %v", usbguardJob, err)
		}
	}

	runTest(true /*usbguardEnabled*/, false /*usbbouncerEnabled*/)
	runTest(false /*usbguardEnabled*/, false /*usbbouncerEnabled*/)
	// Testing USB Bouncer requires the usb_bouncer ebuild which isn't installed by default yet.
}
