// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"

	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/security/seccomp"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBGuard,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check that USBGuard-related feature flags work as intended",
		Attr:         []string{"group:mainline", "informational"},
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
		defaultUser = "testuser@gmail.com"
		defaultPass = "testpass"

		usbguardFeature    = "USBGuard"
		usbbouncerFeature  = "USBBouncer"
		usbguardJob        = "usbguard"
		usbguardWrapperJob = "usbguard-wrapper"
		usbguardProcess    = "usbguard-daemon"
		usbguardPolicy     = "/run/usbguard/rules.conf"
		usbguardUID        = 20123
		usbguardGID        = 20123

		seccompPolicyFilename = "usbguard.policy"

		jobTimeout = 10 * time.Second
	)

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
		goal := upstartcommon.StopGoal
		state := upstartcommon.WaitingState
		if running {
			goal = upstartcommon.StartGoal
			state = upstartcommon.RunningState
		}
		err := upstart.WaitForJobStatus(ctx, usbguardJob, goal, state, upstart.TolerateWrongGoal, jobTimeout)
		if err != nil {
			return errors.Wrapf(err, "unexpected job status for %v", usbguardJob)
		}

		if running {
			if _, err := os.Stat(usbguardPolicy); err != nil {
				return errors.Wrapf(err, "failed finding policy %v", usbguardPolicy)
			}
		} else if !onLockScreen {
			err := upstart.WaitForJobStatus(ctx, usbguardWrapperJob, goal, state, upstart.TolerateWrongGoal, jobTimeout)
			if err != nil {
				return errors.Wrapf(err, "failed to wait on job %v to stop", usbguardWrapperJob)
			}
			if _, err := os.Stat(usbguardPolicy); err == nil {
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

		if err := unix.Kill(pid, unix.SIGKILL); err != nil {
			err = errors.Wrapf(err, "failed to kill %v(%v)", usbguardProcess, pid)
		}
		return err
	}

	generateSeccompPolicy := func() {
		// Run daemon with system call logging.

		// Setup /run/usbguard/rules.conf
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

		// Setup daemon command.
		daemonLog := filepath.Join(s.OutDir(), "daemon-strace.log")
		cmd := seccomp.CommandContext(ctx, daemonLog, usbguardProcess, "-s")

		stdoutFile, err := os.Create(filepath.Join(s.OutDir(), "subcommand.stdout"))
		if err != nil {
			s.Fatal("Create(.../subcommand.stdout) failed: ", err)
		}
		defer func() {
			if err := stdoutFile.Close(); err != nil {
				s.Error("stdoutFile.Close() failed: ", err)
			}
		}()
		cmd.Stdout = stdoutFile

		stderrFile, err := os.Create(filepath.Join(s.OutDir(), "subcommand.stderr"))
		if err != nil {
			s.Fatal("Create(.../subcommand.stderr) failed: ", err)
		}
		defer func() {
			if err := stderrFile.Close(); err != nil {
				s.Error("stderrFile.Close() failed: ", err)
			}
		}()
		cmd.Stderr = stderrFile

		// Execute daemon command.
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
			// Wait will always return an error here so we don't care.
			cmd.Wait()
		}()

		// Set up a timer to kill the daemon after one second.
		timer := time.AfterFunc(1*time.Second, func() {
			s.Log("Terminating subprocess")
			if err := cmd.Signal(unix.SIGTERM); err != nil {
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

		// Generate and persist seccomp policy.
		policyFile := filepath.Join(s.OutDir(), seccompPolicyFilename)
		if err := ioutil.WriteFile(policyFile, []byte(m.GeneratePolicy()), 0644); err != nil {
			s.Fatal("Failed to record seccomp policy: ", err)
		}
		s.Logf("Wrote usbguard seccomp policy to %q", policyFile)
	}

	generateSeccompPolicy()

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: defaultUser, Pass: defaultPass}))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome: ", err)
		}
	}()

	if err := expectUsbguardRunning(false /*running*/, false /*onLockScreen*/); err != nil {
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
			s.Error("Failed to close session manager ScreenIsLocked watcher: ", err)
		}
	}()

	s.Log("Locking the screen")
	if err := sm.LockScreen(ctx); err != nil {
		s.Error("Failed to lock the screen: ", err)
		return
	}

	s.Log("Verifying the usbguard job is running")
	if err := expectUsbguardRunning(true /*running*/, true /*onLockScreen*/); err != nil {
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

	if locked, err := sm.IsScreenLocked(ctx); err != nil {
		s.Error("Failed to get lock screen state: ", err)
		return
	} else if !locked {
		s.Error("Got unlocked; expected locked")
		return
	}

	s.Logf("Killing %q to check for respawn", usbguardProcess)
	if err = checkUsbguardRespawn(); err != nil {
		s.Errorf("Failed to check that %v job respawns: %v", usbguardJob, err)
	}

	// Watch for the ScreenIsUnlocked signal.
	swUnlocked, err := sm.WatchScreenIsUnlocked(ctx)
	if err != nil {
		s.Error("Failed to observe the lock screen being dismissed: ", err)
		return
	}
	defer func() {
		if err := swUnlocked.Close(ctx); err != nil {
			s.Error("Failed to close session manager ScreenIsUnlocked watcher: ", err)
		}
	}()

	s.Log("Unlocking the screen")
	if err := unlockScreen(); err != nil {
		s.Error("Failed to unlock the screen: ", err)
		return
	}

	s.Log("Verifying the lock screen is dismissed")
	select {
	case <-swUnlocked.Signals:
		s.Log("Got ScreenIsUnlocked signal")
	case <-ctx.Done():
		s.Error("Didn't get ScreenIsUnlocked signal: ", ctx.Err())
		return
	}

	if err := expectUsbguardRunning(false /*running*/, false /*onLockScreen*/); err != nil {
		s.Errorf("Unexpected final job status for %q: %v", usbguardJob, err)
	}

	if locked, err := sm.IsScreenLocked(ctx); err != nil {
		s.Error("Failed to get lock screen state: ", err)
		return
	} else if locked {
		s.Error("Got locked; expected unlocked")
		return
	}
}
