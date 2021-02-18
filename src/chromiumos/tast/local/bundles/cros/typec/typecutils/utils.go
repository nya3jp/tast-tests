// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package typecutils contains functionality shared by typec tests.
package typecutils

import (
	"context"
	"io/ioutil"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
)

// CheckTBTDevice is a helper function which checks for a TBT connected device.
// |expected| specifies whether we want to check for the presence of a TBT device (true) or the
// absence of one (false).
func CheckTBTDevice(expected bool) error {
	files, err := ioutil.ReadDir("/sys/bus/thunderbolt/devices")
	if err != nil {
		return errors.Wrap(err, "couldn't read TBT devices directory")
	}

	found := ""
	for _, file := range files {
		// Check for non-built-in TBT devices.
		if file.Name() != "domain0" && file.Name() != "0-0" {
			found = file.Name()
			break
		}
	}

	if expected && found == "" {
		return errors.New("no external TBT device found")
	} else if !expected && found != "" {
		return errors.Errorf("found TBT device: %s", found)
	}

	return nil
}

// FindConnectedDPMonitor checks the following two conditions:
// - that modetest indicates a connected Display Port connector
// - that there is a enabled "non-internal" display.
//
// These two signals are used as to determine whether a DP monitor is successfully connected and showing the extended screen.
func FindConnectedDPMonitor(ctx context.Context, tc *chrome.TestConn) error {
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

// CheckPortForTBTPartner checks whether the device has a connected Thunderbolt device.
// We use the 'ectool typecdiscovery' command to accomplish this.
// If |port| is invalid, the ectool command should return an INVALID_PARAM error.
//
// This functions returns:
// - Whether a TBT device is present at a given port.
// - The error value if the command didn't run, else nil.
func CheckPortForTBTPartner(ctx context.Context, port int) (bool, error) {
	out, err := testexec.CommandContext(ctx, "ectool", "typecdiscovery", strconv.Itoa(port), "0").Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to run ectool command")
	}

	// Look for a TBT SVID in the output. If one doesn't exist, return false.
	return regexp.MatchString(`SVID 0x8087`, string(out))
}
