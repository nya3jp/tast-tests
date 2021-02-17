// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeCrash,
		Desc:         "Checks USB Type C mode switch behaviour when typecd crashes",
		Contacts:     []string{"pmalani@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

// ModeCrash does the following:
// - Login.
// - Validate that TBT alt mode is working correctly.
// - Kill typecd
// - Logout.
// - Validate that the dock in in USB+Dp alt mode.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> Thunderbolt3 (>= Titan Ridge) dock -----> DP monitor.
//      (USB4)
//
func ModeCrash(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome with login: ", err)
	}
	defer cr.Close(ctx)

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	s.Log("Verifying that TBT device enumerated correctly")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir("/sys/bus/thunderbolt/devices")
		if err != nil {
			return err
		}
		for _, file := range files {
			// Check for non-built-in TBT devices.
			if file.Name() != "domain0" && file.Name() != "0-0" {
				return nil
			}
		}

		return errors.New("no external TBT device found")
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify TBT devices connected after login: ", err)
	}

	s.Log("Verifying that DP monitor enumerated correctly")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return findConnectedDPMonitor(ctx, testConn)
	}, &testing.PollOptions{Interval: 200 * time.Millisecond, Timeout: 20 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify DP monitor working after login: ", err)
	}

	s.Log("Killing typecd")
	if err := testexec.CommandContext(ctx, "pkill", "typecd").Run(); err != nil {
		s.Fatal("Failed to kill typecd: ", err)
	}

	// Wait for typecd to restart.
	if err := testing.Sleep(ctx, 2000*time.Millisecond); err != nil {
		s.Fatal("Sleep failed", err)
	}

	// Log out
	// We accomplish this by simply starting a new Chrome instance.
	cr.Close(ctx)
	cr, err = chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome with login: ", err)
	}
	defer cr.Close(ctx)

	// Get the testConn again, since it's a new login.
	testConn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	s.Log("Verifying that no TBT devices enumerated")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir("/sys/bus/thunderbolt/devices")
		if err != nil {
			return err
		}
		for _, file := range files {
			// Check for built-in TBT devices.
			if file.Name() == "domain0" || file.Name() == "0-0" {
				continue
			}
			return errors.Errorf("found TBT device: %s", file.Name())
		}
		return nil
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify no TBT devices connected at login screen: ", err)
	}

	s.Log("Verifying that DP monitor enumerated correctly")
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return findConnectedDPMonitor(ctx, testConn)
	}, &testing.PollOptions{Interval: 200 * time.Millisecond, Timeout: 20 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify DP monitor working at login screen: ", err)
	}
}
