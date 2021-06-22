// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"encoding/json"
	"io/ioutil"
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
		Desc:         "USB4 device enumeration check after hot plug-unplug",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec", "typec_lab"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"test_config.json"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIp"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Pre:          chrome.LoggedIn(),
	})
}

// Hot plug-unplug performs the following:
// - Login to chrome.
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
	cSwitchPort := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIp := s.RequiredVar("typec.domainIp")

	//Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Read json config file.
	json_data, err := ioutil.ReadFile(s.DataPath(testConfig))
	if err != nil {
		s.Fatalf("Failed to open %v file : %v", testConfig, err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(json_data, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// checking for USB4 config data.
	usb4Val, ok := data["USB4"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to found USB4 config data in JSON file")
	}

	// Create C-Switch session that performs hot plug-unplug on USB4 device.
	sessionId, err := cswitch.CreateSession(ctx, domainIp)
	if err != nil {
		s.Fatal("Failed to create cswitch session: ", err)
	}

	defer func() {
		s.Log("Cleanup..")
		if err := cswitch.CloseSession(cleanupCtx, sessionId, domainIp); err != nil {
			s.Log("Failed to close cswitch session: ", err)
		}
	}()

	// Enabling cswitch port to perform USB4 hot plug.
	if err := cswitch.EnableCSwitchPort(ctx, sessionId, cSwitchPort, domainIp); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	// Verifying USB4 device name.
	if err := cswitch.IsDeviceEnumerated(ctx, usb4Val["device_name"].(string), tbtPort); err != nil {
		s.Fatal("Failed to match device name: ", err)
	}

	// Verifying USB4 device rx_speed and tx_speed.
	if err := cswitch.TxRxSpeed(ctx, tbtPort, usb4Val["tx_speed"].(string), usb4Val["rx_speed"].(string)); err != nil {
		s.Fatal("Failed to match tx/rx speed: ", err)
	}

	// Verifying USB4 device nvme version.
	if err := cswitch.NvmeVersion(ctx, tbtPort, usb4Val["nvme_version"].(string)); err != nil {
		s.Fatal("Failed to match nvme version: ", err)
	}

	// Verifying USB4 device nvme generation.
	if err = cswitch.Generation(ctx, tbtPort, usb4Val["generation"].(string)); err != nil {
		s.Fatal("Failed to match generation: ", err)
	}

	// Disabling cswitch port to perform USB4 hot unplug.
	if err := cswitch.DisableCSwitchPort(ctx, sessionId, domainIp); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	// Validating USB4 device enumeration.
	if err := cswitch.IsDeviceEnumerated(ctx, usb4Val["device_name"].(string), tbtPort); err == nil {
		s.Fatal("Device still detecting after unplug")
	}
}
