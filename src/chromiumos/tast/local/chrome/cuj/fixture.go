// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/logsaver"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

const (
	// CPUCoolDownTimeout is the time to wait for cpu cool down.
	CPUCoolDownTimeout = 10 * time.Minute
	// CPUIdleTimeout is the time to wait for cpu utilization to go down.
	// This value should match waitIdleCPUTimeout in cpu/idle.go.
	CPUIdleTimeout = 2 * time.Minute
	// CPUStablizationTimeout is the time to wait for cpu stablization, which
	// is the sum of cpu cool down time and cpu idle time.
	CPUStablizationTimeout = CPUCoolDownTimeout + CPUIdleTimeout

	resetTimeout = 30 * time.Second

	webRTCEventLogCommandFlag = "--webrtc-event-logging=/tmp"
	webRTCEventLogFilePattern = "/tmp/event_log_*.log"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "prepareForCUJ",
		Desc: "The fixture to prepare DUT for CUJ tests",
		Contacts: []string{
			"xiyuan@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl:           &prepareCUJFixture{},
		PreTestTimeout: CPUStablizationTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "cpuIdleForCUJ",
		Desc: "The fixture to wait DUT cpu to idle for CUJ tests",
		Contacts: []string{
			"jane.yang@cienet.com",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl:           &cpuIdleForCUJFixture{},
		PreTestTimeout: CPUIdleTimeout + 5*time.Second,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "cpuIdleForEnrolledCUJ",
		Desc: "The fixture to wait DUT cpu to idle for logged in with gaia user on an enrolled device",
		Contacts: []string{
			"alston.huang@cienet.com",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl:            &cpuIdleForCUJFixture{},
		PreTestTimeout:  CPUIdleTimeout + 5*time.Second,
		SetUpTimeout:    chrome.EnrollmentAndLoginTimeout + chrome.GAIALoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		PostTestTimeout: 15 * time.Second,
		Parent:          fixture.Enrolled,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToCUJUser",
		Desc: "The main fixture used for UI CUJ tests",
		Contacts: []string{
			"xiyuan@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: &loggedInToCUJUserFixture{
			chromeExtraOpts: []chrome.Option{
				chrome.EnableFeatures("WebUITabStrip", "WebUITabStripTabDragIntegration"),
			},
			bt: browser.TypeAsh,
		},
		Parent:          "prepareForCUJ",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cujAccountPool"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToCUJUserEnterpriseWithWebRTCEventLogging",
		Desc: "The main fixture used for UI CUJ tests using an enterprise account, with WebRTC event logging",
		Contacts: []string{
			"xiyuan@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: &loggedInToCUJUserFixture{
			chromeExtraOpts:   []chrome.Option{chrome.ExtraArgs(webRTCEventLogCommandFlag)},
			bt:                browser.TypeAsh,
			useEnterprisePool: true,
		},
		Parent:          "prepareForCUJ",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cujEnterpriseAccountPool"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInAndKeepState",
		Desc: "The CUJ test fixture which keeps login state",
		Contacts: []string{
			"xiyuan@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: &loggedInToCUJUserFixture{
			keepState:       true,
			chromeExtraOpts: []chrome.Option{chrome.EnableFeatures("WebUITabStrip")},
			bt:              browser.TypeAsh,
		},
		Parent:          "cpuIdleForCUJ",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cujAccountPool"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToCUJUserLacros",
		Desc: "Fixture used for lacros variation of UI CUJ tests",
		Contacts: []string{
			"xiyuan@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl:            &loggedInToCUJUserFixture{bt: browser.TypeLacros},
		Parent:          "cpuIdleForCUJ",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cujAccountPool"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInAndKeepStateLacros",
		Desc: "Fixture keeping login status and used for lacros variation of CUJ tests",
		Contacts: []string{
			"xliu@cienet.com",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: &loggedInToCUJUserFixture{
			keepState: true,
			chromeExtraOpts: []chrome.Option{
				chrome.EnableFeatures("WebUITabStrip"),
				chrome.LacrosEnableFeatures("WebUITabStrip"),
			},
			bt: browser.TypeLacros,
		},
		Parent:          "cpuIdleForCUJ",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cujAccountPool"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "enrolledLoggedInToCUJUser",
		Desc: "Logged in with gaia user on an enrolled device",
		Contacts: []string{
			"alston.huang@cienet.com",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: &loggedInToCUJUserFixture{
			chromeExtraOpts: []chrome.Option{chrome.EnableFeatures("WebUITabStrip")},
		},
		Parent:          "cpuIdleForEnrolledCUJ",
		SetUpTimeout:    chrome.EnrollmentAndLoginTimeout + chrome.GAIALoginTimeout + optin.OptinTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.cujAccountPool",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "enrolledLoggedInToCUJUserLacros",
		Desc: "Logged in with gaia user on an enrolled device and used for lacros variation of CUJ tests",
		Contacts: []string{
			"jane.yang@cienet.com",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: &loggedInToCUJUserFixture{
			chromeExtraOpts: []chrome.Option{
				chrome.EnableFeatures("WebUITabStrip"),
				chrome.LacrosEnableFeatures("WebUITabStrip"),
			},
			bt: browser.TypeLacros,
		},
		Parent:          "cpuIdleForEnrolledCUJ",
		SetUpTimeout:    chrome.EnrollmentAndLoginTimeout + chrome.GAIALoginTimeout + optin.OptinTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.cujAccountPool",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToCUJUserWithWebRTCEventLogging",
		Desc: "CUJ test fixture with WebRTC event logging",
		Contacts: []string{
			"amusbach@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: &loggedInToCUJUserFixture{
			chromeExtraOpts: []chrome.Option{chrome.ExtraArgs(webRTCEventLogCommandFlag)},
			bt:              browser.TypeAsh,
		},
		Parent:          "prepareForCUJ",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cujAccountPool"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "loggedInToCUJUserWithWebRTCEventLoggingLacros",
		Desc: "Lacros variation of loggedInToCUJUserWithWebRTCEventLogging",
		Contacts: []string{
			"amusbach@chromium.org",
			"chromeos-perfmetrics-eng@google.com",
		},
		Impl: &loggedInToCUJUserFixture{
			chromeExtraOpts: []chrome.Option{chrome.LacrosExtraArgs(webRTCEventLogCommandFlag)},
			bt:              browser.TypeLacros,
		},
		Parent:          "cpuIdleForCUJ",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{"ui.cujAccountPool"},
	})
}

func loginOption(s *testing.FixtState, useEnterprisePool bool) chrome.Option {
	var variableName string

	if useEnterprisePool {
		variableName = "ui.cujEnterpriseAccountPool"
	} else {
		variableName = "ui.cujAccountPool"
	}

	return chrome.GAIALoginPool(s.RequiredVar(variableName))
}

func runningPackages(ctx context.Context, a *arc.ARC) (map[string]struct{}, error) {
	tasks, err := a.TaskInfosFromDumpsys(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing activities failed")
	}
	acts := make(map[string]struct{})
	for _, t := range tasks {
		for _, a := range t.ActivityInfos {
			acts[a.PackageName] = struct{}{}
		}
	}
	return acts, nil
}

// CPUCoolDownConfig returns a cpu.CoolDownConfig to be used for CUJ tests.
func CPUCoolDownConfig() cpu.CoolDownConfig {
	cdConfig := cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)
	cdConfig.PollTimeout = CPUCoolDownTimeout
	return cdConfig
}

type prepareCUJFixture struct{}

func (f *prepareCUJFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return nil
}

func (f *prepareCUJFixture) TearDown(ctx context.Context, s *testing.FixtState) {
}

func (f *prepareCUJFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *prepareCUJFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Ensure display on to record ui performance correctly. Keep trying for 2 min
	// since it could take 2 min for `powerd` dbus service to be accessible via
	// dbus from tast. See b/244752048.
	if err := testing.Poll(ctx, power.TurnOnDisplay, &testing.PollOptions{
		Interval: 10 * time.Second,
		Timeout:  2 * time.Minute,
	}); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Wait for cpu to stabilize before test. Note this only works as expected if
	// all child fixtures's PreTest and the setup in each test main function do
	// not do cpu intensive works. Otherwise, this needs to moved into body of
	// tests.
	if err := cpu.WaitUntilStabilized(ctx, CPUCoolDownConfig()); err != nil {
		// Log the cpu stabilizing wait failure instead of make it fatal.
		// TODO(b/213238698): Include the error as part of test data.
		s.Log("Failed to wait for CPU to become idle: ", err)
	}
}

func (f *prepareCUJFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

type cpuIdleForCUJFixture struct{}

func (f *cpuIdleForCUJFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return nil
}

func (f *cpuIdleForCUJFixture) TearDown(ctx context.Context, s *testing.FixtState) {
}

func (f *cpuIdleForCUJFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *cpuIdleForCUJFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Wait for cpu to idle before test.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		// Log the cpu idle wait failure instead of make it fatal.
		testing.ContextLog(ctx, "Failed to wait for CPU to become idle: ", err)
	}
}

func (f *cpuIdleForCUJFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

// FixtureData is the struct returned by the preconditions.
type FixtureData struct {
	chrome *chrome.Chrome
	ARC    *arc.ARC
}

// Chrome gets the CrOS-chrome instance.
func (f FixtureData) Chrome() *chrome.Chrome { return f.chrome }

type loggedInToCUJUserFixture struct {
	cr              *chrome.Chrome
	arc             *arc.ARC
	origRunningPkgs map[string]struct{}
	logMarker       *logsaver.Marker
	keepState       bool
	chromeExtraOpts []chrome.Option
	// bt describes what type of browser this fixture should use
	bt                browser.Type
	useEnterprisePool bool
}

func (f *loggedInToCUJUserFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var cr *chrome.Chrome

	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()

		opts := []chrome.Option{
			loginOption(s, f.useEnterprisePool),
			chrome.ARCSupported(),
			chrome.ExtraArgs(arc.DisableSyncFlags()...),
			chrome.DisableFeatures("FirmwareUpdaterApp"),
		}
		if f.keepState {
			opts = append(opts, chrome.KeepState())
		}
		opts = append(opts, f.chromeExtraOpts...)

		var err error
		if f.bt == browser.TypeLacros {
			opts, err = lacrosfixt.NewConfig(lacrosfixt.Mode(lacros.LacrosOnly),
				lacrosfixt.ChromeOptions(opts...)).Opts()
			if err != nil {
				s.Fatal("Failed to get lacros options: ", err)
			}
		}
		cr, err = chrome.New(ctx, opts...)

		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		chrome.Lock()
	}()
	defer func() {
		if cr != nil {
			chrome.Unlock()
			if err := cr.Close(ctx); err != nil {
				s.Error("Failed to close Chrome: ", err)
			}
		}
	}()

	enablePlayStore := true
	if f.keepState {
		// Check whether the play store has been enabled.
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API: ", err)
		}
		st, err := arc.GetState(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get ARC state: ", err)
		}
		enablePlayStore = !st.Provisioned
	}

	if enablePlayStore {
		func() {
			const playStorePackageName = "com.android.vending"
			ctx, cancel := context.WithTimeout(ctx, optin.OptinTimeout+time.Minute)
			defer cancel()

			// Optin to Play Store.
			s.Log("Opting into Play Store")
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get the test conn: ", err)
			}
			maxAttempts := 2
			if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
				s.Fatal("Failed to optin to Play Store: ", err)
			}

			s.Log("Waiting for Playstore shown")
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ARCPackageName == playStorePackageName
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				// Playstore app window might not be shown, but optin should be successful
				// at this time. Log the error message but continue.
				s.Log("Failed to wait for the playstore window to be visible: ", err)
				return
			}

			if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
				s.Fatal("Failed to close Play Store: ", err)
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if _, err := ash.GetARCAppWindowInfo(ctx, tconn, playStorePackageName); err == ash.ErrWindowNotFound {
					return nil
				} else if err != nil {
					return testing.PollBreak(err)
				}
				return errors.New("still seeing playstore window")
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Failed to wait for the playstore window to be closed: ", err)
			}
		}()
	}

	var a *arc.ARC
	func() {
		ctx, cancel := context.WithTimeout(ctx, arc.BootTimeout)
		defer cancel()

		var err error
		if a, err = arc.New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}

		if f.origRunningPkgs, err = runningPackages(ctx, a); err != nil {
			if err := a.Close(ctx); err != nil {
				s.Error("Failed to close ARC connection: ", err)
			}
			s.Fatal("Failed to list running packages: ", err)
		}
	}()
	f.cr = cr
	f.arc = a
	cr = nil
	return FixtureData{chrome: f.cr, ARC: f.arc}
}

func (f *loggedInToCUJUserFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()

	if err := f.arc.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close ARC connection: ", err)
	}

	if err := f.cr.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
	}
}

func (f *loggedInToCUJUserFixture) Reset(ctx context.Context) error {
	// Check oauth2 token is still valid. If not, return an error to restart
	// chrome and re-login.
	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the test conn")
	}
	if st, err := lockscreen.GetState(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to get login status")
	} else if !st.HasValidOauth2Token {
		return errors.New("invalid oauth2 token")
	}

	// Stopping the running apps.
	running, err := runningPackages(ctx, f.arc)
	if err != nil {
		return errors.Wrap(err, "failed to get running packages")
	}
	for pkg := range running {
		if _, ok := f.origRunningPkgs[pkg]; ok {
			continue
		}
		testing.ContextLogf(ctx, "Stopping package %q", pkg)
		if err := f.arc.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to stop %q", pkg)
		}
	}

	// Unlike ARC.preImpl, this does not uninstall apps. This is because we
	// typically want to reuse the same list of applications, and additional
	// installed apps wouldn't affect the test scenarios.
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}

	// Ensures that there are no toplevel windows left open.
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to call ash.GetAllWindows")
	} else if len(all) != 0 {
		return errors.Wrapf(err, "toplevel window (%q) stayed open, total %d left", all[0].Name, len(all))
	}

	return nil
}

func (f *loggedInToCUJUserFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if f.logMarker != nil {
		s.Log("A log marker is already created but not cleaned up")
	}
	logMarker, err := logsaver.NewMarker(f.cr.LogFilename())
	if err == nil {
		f.logMarker = logMarker
	} else {
		s.Log("Failed to start the log saver: ", err)
	}

	webRTCLogs, err := filepath.Glob(webRTCEventLogFilePattern)
	if err != nil {
		s.Log("Failed to check for WebRTC event log files before test: ", err)
	}
	if len(webRTCLogs) == 0 {
		return
	}
	s.Log("Deleting WebRTC event log files found in /tmp before test: ", webRTCLogs)
	for _, filename := range webRTCLogs {
		if err := os.Remove(filename); err != nil {
			s.Logf("Failed to delete %q: %s", filename, err)
		}
	}
}

func (f *loggedInToCUJUserFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.logMarker != nil {
		if err := f.logMarker.Save(filepath.Join(s.OutDir(), "chrome.log")); err != nil {
			s.Log("Failed to store per-test log data: ", err)
		}
		f.logMarker = nil
	}

	webRTCLogs, err := filepath.Glob(webRTCEventLogFilePattern)
	if err != nil {
		s.Log("Failed to check for WebRTC event log files after test: ", err)
	}
	if len(webRTCLogs) == 0 {
		return
	}
	s.Log("Gathering WebRTC event log files: ", webRTCLogs)
	for _, filename := range webRTCLogs {
		outFile := filepath.Join(s.OutDir(), filepath.Base(filename))
		if err := copyFileForTestResults(filename, outFile); err != nil {
			s.Logf("Failed to copy %q to %q: %s", filename, outFile, err)
		}
		if err := os.Remove(filename); err != nil {
			s.Logf("Failed to delete %q: %s", filename, err)
		}
	}
}

// copyFileForTestResults copies a file. The destination file must not already exist. The
// created copy has file permissions 0644 regardless of the file permissions of the original.
func copyFileForTestResults(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}
