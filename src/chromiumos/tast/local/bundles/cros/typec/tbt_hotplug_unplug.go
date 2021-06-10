// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	// Config file which contains expected values of TBT parameters
	testConfig = "test_config.json"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TbtHotplugUnplug,
		Desc:         "TBT device enumeration check after hot plug-unplug",
		Contacts:     []string{"pathan.jilani@gmail.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testConfig},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIp"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Pre:          chrome.LoggedIn(),
	})
}

// Hot plug-unplug performs the following:
// - Login to chrome
// - Hot plug TBT Device into DUT with help of cswitch
// - Performs TBT Device enumeration check
// - Unplug TBT device from DUT and validates TBT device detection
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug)---->TBT SSD


// TbtHotplugUnplug func performs hot plug unplug on connected TBT device and validate device enumeration
func TbtHotplugUnplug(ctx context.Context, s *testing.State) {
	// TBT port ID in the DUT
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID
	cSwitchPort := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device
	domainIp := s.RequiredVar("typec.domainIp")

	// Read json config file
	json_data, err := ioutil.ReadFile(s.DataPath(testConfig))
	if err != nil {
		s.Fatal("Failed to read response data", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(json_data, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// checking for TBT config data
	tbtVal,ok:=data["TBT"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to found TBT config data in JSON file")
	}
	
	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionId, err := cswitch.CreateSession(ctx, domainIp)
	if err != nil {
		s.Fatal(err)
	}

	defer func() {
		s.Log("Cleanup..")
		if err := cswitch.CloseSession(ctx, sessionId, domainIp); err != nil {
			s.Fatal(err)
		}
	}()

	// Enabling cswitch port to perform TBT hot plug.
	if err := cswitch.EnableCSwitchPort(ctx, sessionId, cSwitchPort, domainIp); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	// Verifying TBT device name.
	if err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort); err != nil {
		s.Fatal("Failed to match device name: ", err)
	}

    // Verifying TBT device rx_speed.
	if err := cswitch.TxRxSpeed(ctx, tbtPort, tbtVal["tx_speed"].(string), tbtVal["rx_speed"].(string)); err != nil {
		s.Fatal("Failed to match tx/rx speed: ", err)
	}

	// Verifying TBT device nvme version.
	if err := cswitch.NvmeVersion(ctx, tbtPort, tbtVal["nvme_version"].(string)); err != nil {
		s.Fatal("Failed to match nvme version: ", err)
	}

    // Verifying TBT device nvme generation.
	if err = cswitch.Generation(ctx, tbtPort, tbtVal["generation"].(string)); err != nil {
		s.Fatal("Failed to match generation: ", err)
	}
	
	s.Log("TBT device plugged sucessfully")
	
	// Disabling cswitch port to perform TBT hot unplug
	if err := cswitch.DisableCSwitchPort(ctx, sessionId, domainIp); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	// Validating TBT device enumeration
	if err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort); err == nil {
		s.Fatal("Device still detecting after unplug: ", err)
	}
	s.Log("TBT device unplugged sucessfully")
}
