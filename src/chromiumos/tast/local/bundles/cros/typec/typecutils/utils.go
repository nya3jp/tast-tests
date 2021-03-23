// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package typecutils contains functionality shared by typec tests.
package typecutils

import (
	"bytes"
	"context"
	"io/ioutil"
	"regexp"
	"strconv"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
)

// The maximum number of USB Type C ports that a Chromebook supports.
const maxTypeCPorts = 8

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
		if file.Name() == "domain0" || file.Name() == "0-0" {
			continue
		}

		// Check for retimers.
		// They are of the form "0-0:1.1" or "0-0:3.1".
		if matched, err := regexp.MatchString(`[\d\-\:]+\.\d`, file.Name()); err != nil {
			return errors.Wrap(err, "couldn't execute retimer regexp")
		} else if matched {
			continue
		}

		found = file.Name()
		break
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

// CheckPortsForTBTPartner checks whether the device has a connected Thunderbolt device.
// We use the 'ectool typecdiscovery' command to accomplish this.
// Check each port successively. If a port returns an INVALID_PARAM error, that means
// we are out of ports.
//
// This functions returns:
// - Whether a TBT device is connected to the DUT.
// - The error value if the command didn't run, else nil.
func CheckPortsForTBTPartner(ctx context.Context) (bool, error) {
	for i := 0; i < maxTypeCPorts; i++ {
		out, err := testexec.CommandContext(ctx, "ectool", "typecdiscovery", strconv.Itoa(i), "0").CombinedOutput()
		if err != nil {
			// If we get an invalid param error, that means there are no more ports left.
			// In that case, we shouldn't return an error, but should return false.
			//
			// TODO(pmalani): Determine how many ports a device supports, instead of
			// relying on INVALID_PARAM.
			if bytes.Contains(out, []byte("INVALID_PARAM")) {
				return false, nil
			}

			return false, errors.Wrap(err, "failed to run ectool command")
		}

		// Look for a TBT SVID in the output. If one exists, return immediately.
		if bytes.Contains(out, []byte("SVID 0x8087")) {
			return true, nil
		}
	}

	return false, nil
}
