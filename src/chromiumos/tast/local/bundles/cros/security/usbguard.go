// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
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
		defaultUser   = "testuser@gmail.com"
		defaultPass   = "testpass"
		defaultGaiaID = "gaia-id"

		usbguardFeature    = "USBGuard"
		usbbouncerFeature  = "USBBouncer"
		usbguardJob        = "usbguard"
		usbguardWrapperJob = "usbguard-wrapper"
		usbguardProcess    = "usbguard-daemon"
		usbguardPolicy     = "/run/usbguard/rules.conf"

		jobTimeout = 10 * time.Second
	)

	isFeatureEnabled := func(feature string) (bool, error) {
		const (
			dbusName   = "org.chromium.ChromeFeaturesService"
			dbusPath   = "/org/chromium/ChromeFeaturesService"
			dbusMethod = "org.chromium.ChromeFeaturesServiceInterface.IsFeatureEnabled"
		)
		enabled := false

		_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
		if err != nil {
			return enabled, err
		}

		err = obj.CallWithContext(ctx, dbusMethod, 0, feature).Store(&enabled)
		return enabled, err
	}

	lockScreen := func() error {
		const (
			dbusName   = "org.chromium.SessionManager"
			dbusPath   = "/org/chromium/SessionManager"
			dbusMethod = "org.chromium.SessionManagerInterface.LockScreen"
		)

		_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
		if err != nil {
			return errors.Wrap(err, "failed to connect to session_manager")
		}

		if err = obj.CallWithContext(ctx, dbusMethod, 0).Err; err != nil {
			return errors.Wrapf(err, "failed to invoke %q", dbusMethod)
		}

		return nil
	}

	unlockScreen := func() error {
		ew, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to open keyboard device")
		}
		defer ew.Close()

		if err = ew.Type(ctx, defaultPass+"\n"); err != nil {
			return errors.Wrap(err, "failed to type password")
		}
		return nil
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
		_, _, pid, err := upstart.JobStatus(ctx, usbguardJob)
		if err != nil {
			return errors.Wrapf(err, "failed to get %v pid", usbguardJob)
		}
		if pid == 0 {
			return errors.Errorf("no pid for %v", usbguardJob)
		}

		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			err = errors.Wrapf(err, "failed to kill %v(%v)", usbguardProcess, pid)
		}
		return err
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

		cr, err := chrome.New(ctx, chrome.ExtraArgs(args...), chrome.Auth(defaultUser, defaultPass, defaultGaiaID))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		for name, val := range featureValues {
			if en, err := isFeatureEnabled(name); err != nil {
				s.Errorf("Failed checking if the %v feature is enabled: %v", name, err)
				return
			} else if en != val {
				s.Errorf("The %v feature's enabled state is %v; want %v", name, en, val)
				return
			}
		}

		if err = expectUsbguardRunning(false /*running*/, false /*onLockScreen*/); err != nil {
			s.Errorf("Unexpected initial job status for %q: %v", usbguardJob, err)
			return
		}

		// Watch for the ScreenIsLocked signal.
		sm, err := session.NewSessionManager(ctx)
		if err != nil {
			s.Fatal("Failed to connect session_manager: ", err)
		}

		sw, err := sm.WatchScreenIsLocked(ctx)
		if err != nil {
			s.Error("Failed to observe the lock screen being shown: ", err)
			return
		}
		defer sw.Close(ctx)

		s.Log("Locking the screen")
		if err = lockScreen(); err != nil {
			s.Error("Failed to lock the screen: ", err)
			return
		}

		s.Log("Verifying the usbguard job is running")
		if err = expectUsbguardRunning(usbguardEnabled /*running*/, true /*onLockScreen*/); err != nil {
			s.Error("Failed to check if usbguard is running: ", err)
		}

		s.Log("Verifying the lock screen is shown")
		select {
		case <-sw.Signals:
			s.Log("Got ScreenIsLocked signal")
		case <-ctx.Done():
			s.Error("Didn't get ScreenIsLocked signal: ", ctx.Err())
			return
		}

		if usbguardEnabled {
			s.Logf("Killing %q to check for respawn", usbguardProcess)
			if err = checkUsbguardRespawn(); err != nil {
				s.Errorf("Failed to check that %v job respawns: %v", usbguardJob, err)
			}
		}

		s.Log("Unlocking the screen")
		if err = unlockScreen(); err != nil {
			s.Error("Failed to unlock the screen: ", err)
			return
		}
		if err = expectUsbguardRunning(false /*running*/, false /*onLockScreen*/); err != nil {
			s.Errorf("Unexpected final job status for %q: %v", usbguardJob, err)
		}
	}

	runTest(true /*usbguardEnabled*/, false /*usbbouncerEnabled*/)
	runTest(false /*usbguardEnabled*/, false /*usbbouncerEnabled*/)
	// Testing USB Bouncer requires the usb_bouncer ebuild which isn't installed by default yet.
}
