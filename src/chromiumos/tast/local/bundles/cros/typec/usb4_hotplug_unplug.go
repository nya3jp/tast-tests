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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Usb4HotplugUnplug,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "USB4 device enumeration check after hot plug-unplug",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"test_config.json"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Val: 1,
		}, {
			Name:      "stress",
			Val:       500,
			ExtraAttr: []string{"group:stress"},
			Timeout:   3 * time.Hour,
		}},
	})
}

// Hot plug-unplug performs the following:
// - Hot plug USB4 Device into DUT with help of cswitch.
// - Performs USB4 Device enumeration check.
// - Unplug USB4 device from DUT and validates USB4 device detection.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug)---->USB4.

// Usb4HotplugUnplug func performs hot plug unplug on connected USB4 device and validate device enumeration.
func Usb4HotplugUnplug(ctx context.Context, s *testing.State) {
	// Config file which contains expected values of USB4 parameters.
	const testConfig = "test_config.json"
	// TBT port ID in the DUT.
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	// Shorten deadline to leave time for cleanup.
	cleanupCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
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
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create sessionID: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Cleanup")
		if err := cswitch.CloseSession(cleanupCtx, sessionID, domainIP); err != nil {
			s.Log("Failed to close sessionID: ", err)
		}
	}(cleanupCtx)
	numTrial := s.Param().(int)
	for i := 0; i < numTrial; i++ {
		if numTrial > 1 {
			s.Logf("Trial %d/%d", i+1, numTrial)
		}
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
			s.Fatal("Failed to enable c-switch port: ", err)
		}

		connected, err := cswitch.IsDeviceEnumerated(ctx, usb4Val["device_name"].(string), tbtPort)
		if !connected {
			s.Fatal("Failed to enumerate the TBT device: ", err)
		}

		txSpeed, err := cswitch.TxSpeed(ctx, tbtPort)
		if err != nil {
			s.Fatal("Failed to get the txSpeed: ", err)
		}

		if strings.TrimSpace(txSpeed) != usb4Val["tx_speed"].(string) {
			s.Fatalf("Failed to verify the tx speed, got %s, want %s", txSpeed, usb4Val["tx_speed"].(string))
		}

		rxSpeed, err := cswitch.RxSpeed(ctx, tbtPort)
		if err != nil {
			s.Fatal("Failed to get the rxSpeed: ", err)
		}

		if strings.TrimSpace(rxSpeed) != usb4Val["rx_speed"].(string) {
			s.Fatalf("Failed to verify the rx speed, got %s, want %s", rxSpeed, usb4Val["rx_speed"].(string))
		}

		nvmVersion, err := cswitch.NvmVersion(ctx, tbtPort)
		if err != nil {
			s.Fatal("Failed to get the nvmVersion: ", err)
		}

		if strings.TrimSpace(nvmVersion) != usb4Val["nvme_version"].(string) {
			s.Fatalf("Failed to verify the nvme version, got %s, want %s", nvmVersion, usb4Val["nvme_version"].(string))
		}

		tbtGeneration, err := cswitch.Generation(ctx, tbtPort)
		if err != nil {
			s.Fatal("Failed to get the tbtGeneration: ", err)
		}
		if strings.TrimSpace(tbtGeneration) != usb4Val["generation"].(string) {
			s.Fatalf("Failed to verify the generation, got %s, want %s", tbtGeneration, usb4Val["generation"].(string))
		}

		cSwitchOFF := "0"
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}

		isConnected, err := cswitch.IsDeviceEnumerated(ctx, usb4Val["device_name"].(string), tbtPort)
		if isConnected {
			s.Fatal("Failed to disconnect the TBT device: ", err)
		}
	}
}
