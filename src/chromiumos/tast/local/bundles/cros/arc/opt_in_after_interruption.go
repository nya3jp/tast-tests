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
		Timeout:      7 * time.Minute,
		Params: []testing.Param{{
			Name:              "unmanaged",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: optInTestParams{
				username:   "arc.OptInAfterInterruption.unmanaged_username",
				password:   "arc.OptInAfterInterruption.unmanaged_password",
				delays:     []time.Duration{7, 17, 22, 32},
				chromeArgs: []string{},
			},
		}, {
			Name:              "unmanaged_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: optInTestParams{
				username:   "arc.OptInAfterInterruption.unmanaged_username",
				password:   "arc.OptInAfterInterruption.unmanaged_password",
				delays:     []time.Duration{7, 17, 22, 32},
				chromeArgs: []string{"--enable-arcvm"},
			},
		}, {
			Name:              "managed",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: optInTestParams{
				username:   "arc.OptInAfterInterruption.managed_username",
				password:   "arc.OptInAfterInterruption.managed_password",
				delays:     []time.Duration{10, 21, 26, 36},
				chromeArgs: []string{"--arc-force-show-optin-ui"},
			},
		}, {
			Name:              "managed_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: optInTestParams{
				username:   "arc.OptInAfterInterruption.managed_username",
				password:   "arc.OptInAfterInterruption.managed_password",
				delays:     []time.Duration{10, 21, 26, 36},
				chromeArgs: []string{"--arc-force-show-optin-ui", "--enable-arcvm"},
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
// ARC provisioning and then restarts Chrome after some delay. It confirms that
// ARC completes provisioning on next login.  The delay to use for each test
// iteration originates from a pre-defined list.  If the expected list is exhausted,
// the test will fail.  The test passes when ARC is provisioned before Chrome
// restarts.  The pass case indicates that trying longer delays does not make
// sense on this platform because ARC would be provisioning in this interval and
// next Chrome restart will deal with already provisioned ARC and provisioning
// flow won't be initiated.
func OptInAfterInterruption(ctx context.Context, s *testing.State) {
	param := s.Param().(optInTestParams)
	delays := param.delays
	for _, delay := range delays {
		delaySeconds := delay * time.Second
		s.Logf("Start iteration for %v delay", delaySeconds)

		// continueTesting will return true if test should proceed
		// after optin interruption or false if test should exit.
		continueTesting := attemptOptIn(ctx, s, &param, delaySeconds)
		if !continueTesting() {
			// Test ends early if ARC is already provisioned or error.
			break
		}

		s.Logf("End iteration for %v delay", delaySeconds)
	}
}

// checkArcProvisioned checks whether ARC provisioning is completed.
func checkArcProvisioned(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	var provisioned = false
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
// and will return appropriate handler depending on whether provisioning was successful.
func attemptOptIn(ctx context.Context, s *testing.State, p *optInTestParams, t time.Duration) func() bool {
	completedEarly := func() bool {
		s.Logf("Completed ARC provisioning before finishing iteration for %v delay", t)
		return false
	}

	username := s.RequiredVar(p.username)
	password := s.RequiredVar(p.password)
	extraArgs := p.chromeArgs
	firstRun := p.delays[0]

	args := []string{"--arc-disable-app-sync", "--arc-disable-play-auto-install"}
	args = append(args, extraArgs...)

	s.Log("Log into Chrome instance")
	cr, err := chrome.New(
		ctx,
		chrome.ARCSupported(),
		chrome.GAIALogin(),
		chrome.Auth(username, password, ""),
		chrome.ExtraArgs(args...),
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

	s.Log("Check Play Store state")
	playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check Play Store state: ", err)
	} else if playStoreState["allowed"] == false {
		// Sanity check but Play Store should be allowed for accounts in this test.
		s.Error("Invalid response with Play Store state set to not allowed")
		return completedEarly
	}

	s.Log("Performing optin to Play Store (enabling ARC)")
	if err := optin.SetPlayStoreEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set enable Play Store: ", err)
	}

	// Press Agree to continue optin. Managed account is normally tuned to
	// optin silently, but the Chrome arg "--arc-force-show-optin-ui" is
	// used to simplify the unmanaged vs. managed flow.
	s.Log("Show optin UI and accept terms of service")
	if err := optin.FindOptInExtensionPageAndAcceptTerms(ctx, cr, false); err != nil {
		s.Fatal("Failed to find optin extension page: ", err)
	}

	if err := testing.Sleep(ctx, t); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	s.Log("Sleep completed")

	if arcProvisioned, err := checkArcProvisioned(ctx, tconn); err != nil {
		s.Fatal("Failed to check if ARC is provisioned: ", err)
	} else if arcProvisioned {
		s.Logf("ARC is already provisioned after %v delay", t)
		if t == firstRun {
			const warnString = "This is first iteration and Chrome restart " +
				"was not validated. Consider decreasing initial delay value"
			s.Logf("WARNING: %s", warnString)
		}
		return completedEarly
	}
	s.Log("ARC provisioning is not completed")

	// Anonymous function closure accessing ctx, s, username, password, args.
	// Abstracts caller from deciding whether to restart Chrome after
	// interruption and check provisioning completion or end testing.
	return func() bool {
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
		waitForArcProvisioned(ctx, tconn)

		if err := waitForArcProvisioned(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for ARC to complete provisioning: ", err)
		}
		s.Log("ARC is provisioned after restart")

		return true
	}
}
