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
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"

	"chromiumos/policy/enterprise_management"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

// The maximum number of USB Type C ports that a Chromebook supports.
const maxTypeCPorts = 8

// List of built-in Thunderbolt devices enumerated by the OS.
var builtInTBTDevices = []string{"domain0", "domain1", "0-0", "1-0"}

// BuiltInTBTDevice returns whether the specified name is a built-in Thunderbolt device or not.
func BuiltInTBTDevice(name string) bool {
	for _, device := range builtInTBTDevices {
		if name == device {
			return true
		}
	}

	return false
}

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
		if BuiltInTBTDevice(file.Name()) {
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

// CheckTBTAndDP is a convenience function that checks for TBT and DP enumeration.
// It returns nil on success, and the relevant error otherwise.
func CheckTBTAndDP(ctx context.Context, tc *chrome.TestConn) error {
	tbtPollOptions := testing.PollOptions{Timeout: 10 * time.Second}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return CheckTBTDevice(true)
	}, &tbtPollOptions); err != nil {
		return errors.Wrap(err, "failed to verify TBT devices connected")
	}

	dpPollOptions := testing.PollOptions{Timeout: 20 * time.Second}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return FindConnectedDPMonitor(ctx, tc)
	}, &dpPollOptions); err != nil {
		return errors.Wrap(err, "failed to verify DP monitor working")
	}

	return nil
}

// CheckPortsForTBTPartner checks whether the device has a connected Thunderbolt device.
// We use the 'ectool typecdiscovery' command to accomplish this.
// Check each port successively. If a port returns an INVALID_PARAM error, that means
// we are out of ports.
//
// This functions returns:
// - Whether a TBT device is connected to the DUT. If yes, return the port index, otherwise return -1.
// - The error value if the command didn't run, else nil.
func CheckPortsForTBTPartner(ctx context.Context) (int, error) {
	for i := 0; i < maxTypeCPorts; i++ {
		out, err := testexec.CommandContext(ctx, "ectool", "typecdiscovery", strconv.Itoa(i), "0").CombinedOutput()
		if err != nil {
			// If we get an invalid param error, that means there are no more ports left.
			// In that case, we shouldn't return an error, but should return false.
			//
			// TODO(pmalani): Determine how many ports a device supports, instead of
			// relying on INVALID_PARAM.
			if bytes.Contains(out, []byte("INVALID_PARAM")) {
				return -1, nil
			}

			return -1, errors.Wrap(err, "failed to run ectool command")
		}

		// Look for a TBT SVID in the output. If one exists, return immediately.
		if bytes.Contains(out, []byte("SVID 0x8087")) {
			return i, nil
		}
	}

	return -1, nil
}

// buildTestSettings is a helper function which returns a ChromeDeviceSettingsProto with the
// DevicePciPeripheralDataAccessEnabled setting set to true.
func buildTestSettings() *enterprise_management.ChromeDeviceSettingsProto {
	boolTrue := true
	return &enterprise_management.ChromeDeviceSettingsProto{
		DevicePciPeripheralDataAccessEnabledV2: &enterprise_management.DevicePciPeripheralDataAccessEnabledProtoV2{
			Enabled: &boolTrue,
		},
	}
}

// EnablePeripheralDataAccess sets the Chrome device settings to have the "DevicePciPeripheralDataAccessEnabled"
// setting set to true. This allows alternate mode switching to occur. keyPath denotes the keypath of the keyfile
// which is pushed to the device and which is necessary to store the new settings proto.
func EnablePeripheralDataAccess(ctx context.Context, keyPath string) error {
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create session_manager binding")
	}

	privKey, err := session.ExtractPrivKey(keyPath)
	if err != nil {
		return errors.Wrap(err, "failed to parse PKCS #12 file")
	}

	settings := buildTestSettings()
	if err := session.StoreSettings(ctx, sm, "", privKey, nil, settings); err != nil {
		return errors.Wrap(err, "failed to store settings")
	}

	if retrieved, err := session.RetrieveSettings(ctx, sm); err != nil {
		return errors.Wrap(err, "failed to retrieve settings")
	} else if !proto.Equal(retrieved, settings) {
		return errors.Errorf("unexpected settings retrieved, diff: %s", cmp.Diff(retrieved, settings))
	}

	return nil
}
