// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// Timeout to wait ARC provisioning is completed.
	arcProvisionedWaitTimeOut = 60 * time.Second

	// Interval to check ARC provisioning status.
	arcProvisionedCheckInterval = 1 * time.Second
)

type optInTestParams struct {
	username   string          // username for Chrome login.
	password   string          // password to login.
	delays     []time.Duration // slice of delays for Chrome restart in seconds.
	chromeArgs []string        // Arguments to pass to Chrome command line.
}

// getUnmangedDelays is an initializer function that returns delays for unmanaged account tests
func getUnmangedDelays() []time.Duration {
	return []time.Duration{7 * time.Second, 17 * time.Second, 22 * time.Second, 32 * time.Second}
}

// getManagedDelays is an initializer function that returns delays for managed account tests
func getManagedDelays() []time.Duration {
	return []time.Duration{10 * time.Second, 21 * time.Second, 26 * time.Second, 36 * time.Second}
}

func init() {
	testing.AddTest(&testing.Test{
		Func: OptInAfterInterruption,
		Desc: "Verify ARC Provisioning completes even with interruptions by restarting Chrome",
		Contacts: []string{
			"arc-performance@google.com",
			"alanding@chromium.org", // Tast port author.
			"khmel@chromium.org",    // Original autotest author.
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Name:              "unmanaged",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: optInTestParams{
				username:   "arc.OptInAfterInterruption.unmanaged_username",
				password:   "arc.OptInAfterInterruption.unmanaged_password",
				delays:     getUnmangedDelays(),
				chromeArgs: []string{},
			},
		}, {
			Name:              "unmanaged_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: optInTestParams{
				username:   "arc.OptInAfterInterruption.unmanaged_username",
				password:   "arc.OptInAfterInterruption.unmanaged_password",
				delays:     getUnmangedDelays(),
				chromeArgs: []string{},
			},
		}, {
			Name:              "managed",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: optInTestParams{
				username:   "arc.OptInAfterInterruption.managed_username",
				password:   "arc.OptInAfterInterruption.managed_password",
				delays:     getManagedDelays(),
				chromeArgs: []string{"--arc-force-show-optin-ui"},
			},
		}, {
			Name:              "managed_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: optInTestParams{
				username:   "arc.OptInAfterInterruption.managed_username",
				password:   "arc.OptInAfterInterruption.managed_password",
				delays:     getManagedDelays(),
				chromeArgs: []string{"--arc-force-show-optin-ui"},
			},
		}},
		Vars: []string{
			"arc.OptInAfterInterruption.unmanaged_username",
			"arc.OptInAfterInterruption.unmanaged_password",
			"arc.OptInAfterInterruption.managed_username",
			"arc.OptInAfterInterruption.managed_password",
		},
	})
}

// OptInAfterInterruption verifies ARC provisioning is completed after interruption.
//
// This test runs for multiple iterations. On each iteration it starts initial
// ARC provisioning and then restarts Chrome after some delay. It confirms that ARC
// completes provisioning on next login.  The delays used for each test iteration
// are based on cheets_OptInAfterInterruption autotest data collected during NYC.
// If the expected delays list is exhausted, the test will fail.  The test passes
// when ARC is provisioned before Chrome restarts.  The pass case indicates that
// trying longer delays does not make sense on this platform because ARC would be
// provisioning in this interval and next Chrome restart will deal with already
// provisioned ARC and provisioning flow won't be initiated.
func OptInAfterInterruption(ctx context.Context, s *testing.State) {
	param := s.Param().(optInTestParams)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	extraArgs := param.chromeArgs

	args := []string{"--arc-disable-app-sync", "--arc-disable-play-auto-install", "--ignore-arcvm-dev-conf"}
	args = append(args, extraArgs...)

	for _, delay := range param.delays {
		s.Logf("Start iteration for %v delay", delay)

		if continueTesting, err := attemptOptIn(ctx, username, password, args, delay, param.delays[0]); err != nil {
			s.Fatal("Failed to attempting to complete optin: ", err)
		} else if !continueTesting {
			s.Logf("Completed ARC provisioning before finishing iteration for %v delay", delay)
			break
		}

		func() {
			// ARC should start automatically due acceptance above.
			// chrome.KeepState() is set to prevent data clean-up.
			s.Log("Log into another Chrome instance")
			cr, err := chrome.New(
				ctx,
				chrome.ARCSupported(),
				chrome.GAIALogin(),
				chrome.Auth(username, password, ""),
				chrome.ExtraArgs(args...),
				chrome.KeepState(),
			)
			if err != nil {
				s.Fatal("Failed to connect to Chrome: ", err)
			}
			defer cr.Close(ctx)

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create test API connection: ", err)
			}
			defer tconn.Close()

			s.Log("Wait for ARC to complete provisioning")
			if err := waitForArcProvisioned(ctx, tconn); err != nil {
				s.Fatal("Failed to wait for ARC to complete provisioning: ", err)
			}
			s.Log("ARC is provisioned after restart")
		}()
		s.Logf("End iteration for %v delay", delay)
	}
}

// checkArcProvisioned checks whether ARC provisioning is completed.
func checkArcProvisioned(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	var provisioned bool
	if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.isArcProvisioned)()`, &provisioned); err != nil {
		return false, errors.Wrap(err, "failed running autotestPrivate.isArcProvisioned")
	}
	return provisioned, nil
}

// waitForArcProvisioned waits until ARC provisioning is completed.
func waitForArcProvisioned(ctx context.Context, tconn *chrome.TestConn) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if provisioned, err := checkArcProvisioned(ctx, tconn); err != nil {
			return testing.PollBreak(err)
		} else if !provisioned {
			return errors.New("provisioning for ARC is not complete yet")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  arcProvisionedWaitTimeOut,
		Interval: arcProvisionedCheckInterval,
	})
	if err != nil {
		return errors.Wrap(err, "failed to wait for ARC provisioning to complete")
	}
	return nil
}

// attemptOptIn tries to complete ARC provisioning within the specified delay
func attemptOptIn(ctx context.Context, username, password string, args []string, delay, firstRun time.Duration) (bool, error) {
	testing.ContextLog(ctx, "Log into Chrome instance")
	cr, err := chrome.New(
		ctx,
		chrome.ARCSupported(),
		chrome.GAIALogin(),
		chrome.Auth(username, password, ""),
		chrome.ExtraArgs(args...),
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to create test API connection")
	}
	defer tconn.Close()

	testing.ContextLog(ctx, "Check Play Store state")
	if playStoreState, err := optin.GetPlayStoreState(ctx, tconn); err != nil {
		errors.Wrap(err, "failed to check Play Store state")
	} else if playStoreState["allowed"] == false {
		// Sanity check in case Play Store account settings or accounts changed.
		return false, errors.New("invalid response with Play Store state set to not allowed")
	}

	testing.ContextLog(ctx, "Performing optin to Play Store (enabling ARC)")
	if err := optin.SetPlayStoreEnabled(ctx, tconn, true); err != nil {
		return false, errors.Wrap(err, "failed to set enable Play Store")
	}

	// Press Agree to continue optin. Managed account is normally tuned to
	// optin silently, but the Chrome arg "--arc-force-show-optin-ui" is
	// used to simplify the unmanaged vs. managed flow.
	testing.ContextLog(ctx, "Show optin UI and accept terms of service")
	if err := optin.FindOptInExtensionPageAndAcceptTerms(ctx, cr, false); err != nil {
		return false, errors.Wrap(err, "failed to find optin extension page")
	}

	if err := testing.Sleep(ctx, delay); err != nil {
		return false, errors.Wrap(err, "failed to sleep")
	}
	testing.ContextLog(ctx, "Sleep completed")

	if arcProvisioned, err := checkArcProvisioned(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "failed to check if ARC is provisioned")
	} else if arcProvisioned {
		testing.ContextLogf(ctx, "ARC is already provisioned after %v delay", delay)
		if delay == firstRun {
			const warnString = "This is first iteration and Chrome restart " +
				"was not validated. Consider decreasing initial delay value"
			testing.ContextLogf(ctx, "WARNING: %s", warnString)
		}
		return false, nil
	}
	testing.ContextLog(ctx, "ARC provisioning is not completed")
	return true, nil
}
