// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	pow "chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChargingThroughUSB4,
		Desc:         "Checking device charging status after USB4 hot plug-unplug",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"test_config.json"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIp"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Pre:          chrome.LoggedIn(),
	})
}

// ChargingThroughUSB4 performs the following:
// - Pre-requisite : Remove the charger from DUT and make sure USB4 is connected.
// - Hot plug USB4 Device into DUT with help of cswitch.
// - Performs USB4 Device enumeration check.
// - Check DUT charging state.
// - Unplug USB4 device from DUT and validates USB4 device removal detection.
// - Check DUT charging state.
// This test requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug)---->USB4.

// ChargingThroughUSB4 func validates DUT charging state on USB4 hot plug-unplug.
func ChargingThroughUSB4(ctx context.Context, s *testing.State) {
	// Config file which contains expected values of USB4 parameters.
	const testConfig = "test_config.json"
	// TBT port ID in the DUT.
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIp := s.RequiredVar("typec.domainIp")

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Read json config file.
	jsonData, err := ioutil.ReadFile(s.DataPath(testConfig))
	if err != nil {
		s.Fatalf("Failed to open %v file : %v", testConfig, err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for USB4 config data.
	usb4Val, ok := data["USB4"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to found USB4 config data in JSON file")
	}

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionId, err := cswitch.CreateSession(ctx, domainIp)
	if err != nil {
		s.Fatal(err)
	}

	defer func() {
		s.Log("Cleanup..")
		if err := cswitch.CloseSession(cleanupCtx, sessionId, domainIp); err != nil {
			s.Log(err)
		}
	}()

	if err := cswitch.ToggleCSwitchPort(ctx, sessionId, cSwitchON, domainIp); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	connected, err := cswitch.IsDeviceEnumerated(ctx, usb4Val["device_name"].(string), tbtPort)
	if !connected {
		s.Fatal(err)
	}

	txSpeed, err := cswitch.TxSpeed(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(txSpeed) != usb4Val["tx_speed"].(string) {
		s.Fatalf("Failed to verify the tx speed, got %s, want %s", txSpeed, usb4Val["tx_speed"].(string))
	}

	rxSpeed, err := cswitch.RxSpeed(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(rxSpeed) != usb4Val["rx_speed"].(string) {
		s.Fatalf("Failed to verify the rx speed, got %s, want %s", rxSpeed, usb4Val["rx_speed"].(string))
	}

	nvmVersion, err := cswitch.NvmVersion(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(nvmVersion) != usb4Val["nvme_version"].(string) {
		s.Fatalf("Failed to verify the nvme version, got %s, want %s", nvmVersion, usb4Val["nvme_version"].(string))
	}

	tbtGeneration, err := cswitch.Generation(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}
	if strings.TrimSpace(tbtGeneration) != usb4Val["generation"].(string) {
		s.Fatalf("Failed to verify the generation, got %s, want %s", tbtGeneration, usb4Val["generation"].(string))
	}

	// Verifying battery charging status after USB4 hot-plug.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		status, err := pow.GetStatus(ctx)
		if err != nil {
			s.Fatal("Failed to get power status: ", err)
		}
		if status.BatteryDischarging {
			return errors.New("failed to charge after USB4 hot-plug")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Battery status : ", err)
	}

	cSwitchOFF := "0"
	if err := cswitch.ToggleCSwitchPort(ctx, sessionId, cSwitchOFF, domainIp); err != nil {
		s.Fatalf("Failed to disable c-switch port: ", err)
	}

	isConnected, err := cswitch.IsDeviceEnumerated(ctx, usb4Val["device_name"].(string), tbtPort)
	if isConnected {
		s.Fatal(err)
	}

	status, err := pow.GetStatus(ctx)
	if err != nil {
		s.Fatal("Failed to get power status: ", err)
	}
	// Verifying battery discharging after USB4 hot-unplug.
	if !status.BatteryDischarging {
		s.Fatal("Battery failed to discharge after USB4 hot-unplug")
	}
}
