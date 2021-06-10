// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

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
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec", "typec_lab"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testConfig},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIp"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Pre:          chrome.LoggedIn(),
	})
}

// TbtHotplugUnplug performs the following:
// - Login to chrome
// - Hot plug TBT Device into DUT with help of cswitch
// - Performs TBT Device enumeration check
// - Unplug TBT device from DUT and validates TBT device detection
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug)---->TBT SSD
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

	// Checking for TBT config data
	tbtVal, ok := data["TBT"].(map[string]interface{})
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
	if err := cswitch.ToggleCSwitchPort(ctx, sessionId, fmt.Sprintf("cswitch %s", cSwitchPort), domainIp); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	connected, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort)
	if !connected {
		s.Fatal(err)
	}

	txSpeed, err := cswitch.TxSpeed(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(txSpeed) != tbtVal["tx_speed"].(string) {
		s.Fatal("%s is not expected tx speed", txSpeed)
	}

	rxSpeed, err := cswitch.RxSpeed(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(rxSpeed) != tbtVal["rx_speed"].(string) {
		s.Fatal("%s is not expected rx speed", rxSpeed)
	}

	nvmVersion, err := cswitch.NvmVersion(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(nvmVersion) != tbtVal["nvme_version"].(string) {
		s.Fatal("%s is not expected nvme version", nvmVersion)
	}

	tbtGeneration, err := cswitch.Generation(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}
	if strings.TrimSpace(tbtGeneration) != tbtVal["generation"].(string) {
		s.Fatal("%s is not expected generation", tbtGeneration)
	}

	s.Log("TBT device plugged sucessfully")

	// Disabling cswitch port to perform TBT hot unplug
	if err := cswitch.ToggleCSwitchPort(ctx, sessionId, "cswitch 0", domainIp); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	isConnected, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort)
	if isConnected {
		s.Fatal(err)
	}
	s.Log("TBT device unplugged sucessfully")
}
