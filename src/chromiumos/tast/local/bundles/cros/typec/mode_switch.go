// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"io/ioutil"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeSwitch,
		Desc:         "Checks USB Type C mode switch behaviour on login",
		Contacts:     []string{"pmalani@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// ModeSwitch does the following:
// - Go to the login screen.
// - Validate USB+DP alt mode is working correctly.
// - Login.
// - Validate that TBT alt mode is working correctly.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> Thunderbolt3 (>= Titan Ridge) dock -----> DP monitor.
//      (USB4)
//
func ModeSwitch(ctx context.Context, s *testing.State) {
	// Get to the Chrome login screen.
	cr, err := chrome.New(ctx, chrome.DeferLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome at login screen: ", err)
	}
	defer cr.Close(ctx)

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

	// Make sure DP monitor is "working".
	err = testing.Poll(ctx, findConnectedDPMonitor,
		&testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 20 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify DP monitors working at login screen: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
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
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 20 * time.Second})
	if err != nil {
		s.Fatal("Failed to verify TBT devices connected after login: ", err)
	}

	// Validate that DP monitor is "working".
}

func findConnectedDPMonitor(ctx context.Context) error {
	connectors, err := graphics.ModetestConnectors(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get connectors")
	}

	for _, connector := range connectors {
		// We're only interested in DP connectors.
		matched, err := regexp.MatchString(`^DP-\d`, connector.Name)
		if err != nil {
			return err
		}

		if matched && connector.Connected {
			return nil
		}
	}

	return errors.New("no connected DP monitor found")
}
