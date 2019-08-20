// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/security/seccomp"
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
		SoftwareDeps: []string{"chrome", "usbguard"},
		Contacts: []string{
			"allenwebb@chromium.org",
			"jorgelo@chromium.org",
			"chromeos-security@google.com",
		},
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
		usbguardUID        = 20123
		usbguardGID        = 20123

		seccompPolicyFilename = "usbguard.policy"

		dbusName              = "org.usbguard1"
		dbusInterfacePolicy   = "/org/usbguard1/Policy"
		dbusMethodListDevices = "org.usbguard.Policy1.listRules"

		jobTimeout = 10 * time.Second
	)

	isFeatureEnabled := func(feature string) (bool, error) {
		const (
			dbusName   = "org.chromium.ChromeFeaturesService"
			dbusPath   = "/org/chromium/ChromeFeaturesService"
			dbusMethod = "org.chromium.ChromeFeaturesServiceInterface.IsFeatureEnabled"
		)

		_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
		if err != nil {
			return false, err
		}

		enabled := false
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

	unlockScreen := func() (rerr error) {
		ew, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to open keyboard device")
		}
		defer func() {
			if err := ew.Close(); err != nil {
				rerr = errors.Wrap(err, "failed to close keyboard device")
			}
		}()

		if err := ew.Type(ctx, defaultPass+"\n"); err != nil {
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
			} else if !os.IsNotExist(err) {
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
		defer func() {
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome: ", err)
			}
		}()

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
		defer func() {
			if err := sw.Close(ctx); err != nil {
				s.Error("Failed to close session manager client: ", err)
			}
		}()

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

	generateSeccompPolicy := func() {
		// Run daemon with system call logging.
		runDir := filepath.Dir(usbguardPolicy)
		if err := os.MkdirAll(runDir, 0700); err != nil && !os.IsExist(err) {
			s.Errorf("MkdirAll(%q) failed: %v", runDir, err)
		}
		if err := os.Chown(runDir, usbguardUID, usbguardGID); err != nil {
			s.Errorf("Chown(%q) failed: %v", runDir, err)
		}
		if err := ioutil.WriteFile(usbguardPolicy, []byte("allow\n"), 0600); err != nil {
			s.Fatalf("WriteFile(%q): %v", usbguardPolicy, err)
		}
		defer func() {
			if err := os.Remove(usbguardPolicy); err != nil {
				s.Errorf("Remove(%q) failed: %v", usbguardPolicy, err)
			}
		}()
		daemonLog := filepath.Join(s.OutDir(), "daemon-strace.log")
		cmd := seccomp.CommandContext(ctx, daemonLog, usbguardProcess, "-s")
		// Create a sepearate process group for the child so they can be killed as a group.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		var err error
		cmd.Stdout, err = os.Create(filepath.Join(s.OutDir(), "subcommand.stdout"))
		if err != nil {
			s.Fatal("Stdout Create(.../subcommand.stdout) failed: ", err)
		}
		cmd.Stderr, err = os.Create(filepath.Join(s.OutDir(), "subcommand.stderr"))
		if err != nil {
			s.Fatal("Stdout Create(.../subcommand.stderr) failed: ", err)
		}
		if err := cmd.Start(); err != nil {
			s.Fatalf("%q failed with: %v", cmd.Args, err)
		}
		defer func() {
			if cmd.ProcessState != nil {
				// Already waited.
				return
			}
			if err := cmd.Kill(); err != nil {
				s.Error("Failed to kill subcommand: ", err)
			}
			if err := cmd.Wait(); err != nil {
				s.Error("Failed to wait on subcommand: ", err)
			}
		}()

		// Exercise the D-Bus interface
		_, obj, err := dbusutil.Connect(ctx, dbusName, dbusInterfacePolicy)
		if err != nil {
			s.Fatal("D-Bus connection failed: ", err)
		}
		if err := obj.Call(dbusMethodListDevices, 0, "").Err; err != nil {
			s.Fatal("D-Bus method call failed: ", err)
		}
		s.Log("D-Bus method completed")

		// Setup a timer to kill the daemon after one second.
		timer := time.AfterFunc(1*time.Second, func() {
			s.Log("Terminating subprocess")
			if err := cmd.Signal(syscall.SIGTERM); err != nil {
				s.Error("Kill(...) failed: ", err)
			}
		})
		s.Log("Finished recording stdio")

		// Wait for a timeout.
		err = cmd.Wait()
		if timer.Stop() {
			s.Fatalf("%q exited early: %v", usbguardProcess, err)
		}

		// Include results in the policy.
		m := seccomp.NewPolicyGenerator()
		if err := m.AddStraceLog(daemonLog, seccomp.IncludeAllSyscalls); err != nil {
			s.Fatal("AddStraceLog(daemonLog) failed with: ", err)
		}

		// Include "usbguard generate-policy" in the seccomp policy.
		clientLog := filepath.Join(s.OutDir(), "client-strace.log")
		cmd = seccomp.CommandContext(ctx, clientLog, "usbguard", "generate-policy")
		if err := cmd.Run(); err != nil {
			s.Fatalf("%q failed with: %v", cmd.Args, err)
		}
		if err := m.AddStraceLog(clientLog, seccomp.IncludeAllSyscalls); err != nil {
			s.Fatal("AddStraceLog(daemonLog) failed with: ", err)
		}

		// Generate and persist seccomp policy.
		policyFile := filepath.Join(s.OutDir(), seccompPolicyFilename)
		if err := ioutil.WriteFile(policyFile, []byte(m.GeneratePolicy()), 0644); err != nil {
			s.Fatal("Failed to record seccomp policy: ", err)
		}
		s.Logf("Wrote usbguard seccomp policy to %q", policyFile)
	}

	generateSeccompPolicy()
	runTest(true /*usbguardEnabled*/, false /*usbbouncerEnabled*/)
	runTest(false /*usbguardEnabled*/, false /*usbbouncerEnabled*/)
	// Testing USB Bouncer requires the usb_bouncer ebuild which isn't installed by default yet.
}
