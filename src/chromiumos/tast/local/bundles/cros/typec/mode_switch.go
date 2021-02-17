// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"io/ioutil"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// The maximum number of USB Type C ports that a Chromebook supports.
const maxTypeCPorts = 8

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeSwitch,
		Desc:         "Checks USB Type C mode switch behaviour on login",
		Contacts:     []string{"pmalani@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{
			// For running manually.
			{},
			// For automated testing.
			{
				Name:              "test",
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
			}},
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
	// Check if a TBT device is connected. If one isn't, we should skip
	// execution.
	// Check each port successively. If a port returns an error, that means
	// we are out of ports.
	// This check is for test executions which take place on
	// CQ (where TBT peripherals aren't connected).
	for i := 0; i < maxTypeCPorts; i++ {
		if present, err := checkPortForTBTPartner(ctx, i); err != nil {
			s.Log("Couldn't find TBT device from PD identity: ", err)
			return
		} else if present {
			s.Log("Found a TBT device, proceeding with test")
			break
		}
	}

	// Get to the Chrome login screen.
	cr, err := chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome at login screen: ", err)
	}
	defer cr.Close(ctx)

	testConn, err := cr.SigninProfileTestAPIConn(ctx)
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
}

// findConnectedDPMonitor checks the following two conditions:
// - that modetest indicates a connected Display Port connector
// - that there is a enabled "non-internal" display.
//
// These two signals are used as to determine whether a DP monitor is successfully connected and showing the extended screen.
func findConnectedDPMonitor(ctx context.Context, tc *chrome.TestConn) error {
	connectors, err := graphics.ModetestConnectors(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get connectors")
	}

	foundConnected := false
	for _, connector := range connectors {
		// We're only interested in DP connectors.
		matched, err := regexp.MatchString(`^DP-\d`, connector.Name)
		if err != nil {
			return err
		}

		if matched && connector.Connected {
			foundConnected = true
			break
		}
	}

	if !foundConnected {
		return errors.New("no connected DP connector found")
	}

	// Check the DisplayInfo from the Test API connection for a connected extended display.
	infos, err := display.GetInfo(ctx, tc)
	if err != nil {
		return errors.New("failed to get display info from test conn")
	}

	for _, info := range infos {
		if !info.IsInternal && info.IsEnabled {
			return nil
		}
	}

	return errors.New("no enabled and working external display found")
}

// checkPortForTBTPartner checks whether the device has a connected Thunderbolt device.
// We use the 'ectool typecdiscovery' command to accomplish this.
// If |port| is invalid, the ectool command should return an INVALID_PARAM error.
//
// This functions returns:
// - Whether a TBT device is present at a given port.
// - The error value if the command didn't run, else nil.
func checkPortForTBTPartner(ctx context.Context, port int) (bool, error) {
	out, err := testexec.CommandContext(ctx, "ectool", "typecdiscovery", strconv.Itoa(port), "0").Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to run ectool command")
	}

	// Look for a TBT SVID in the output. If one doesn't exist, return false.
	return regexp.MatchString(`SVID 0x8087`, string(out))
}
