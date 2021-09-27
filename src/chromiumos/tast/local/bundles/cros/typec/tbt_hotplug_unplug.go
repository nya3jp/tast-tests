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
	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	// Config file which contains expected values of TBT parameters.
	testConfig = "test_config.json"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TbtHotplugUnplug,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "TBT device enumeration check after hot plug-unplug",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{testConfig, "testcert.p12"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP", "ui.signinProfileTestExtensionManifestKey"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Params: []testing.Param{{
			Val:       1,
			ExtraAttr: []string{"group:mainline"},
		}, {
			Name:      "stress",
			Val:       500,
			ExtraAttr: []string{"group:stress"},
			Timeout:   3 * time.Hour,
		}},
	})
}

// TbtHotplugUnplug performs the following:
// - Hot plug TBT Device into DUT with help of cswitch.
// - Performs TBT Device enumeration check.
// - Unplug TBT device from DUT and validates TBT device detection.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug)---->TBT SSD.
func TbtHotplugUnplug(ctx context.Context, s *testing.State) {
	// TBT port ID in the DUT.
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Get to the Chrome login screen.
	cr, err := chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome at login screen: ", err)
	}
	defer cr.Close(ctx)

	if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
		s.Fatal("Failed to enable peripheral data access setting: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	// Read json config file.
	jsonData, err := ioutil.ReadFile(s.DataPath(testConfig))
	if err != nil {
		s.Fatal("Failed to read response data: ", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for TBT config data.
	tbtVal, ok := data["TBT"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to found TBT config data in JSON file")
	}

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create sessionID: ", err)
	}

	defer func() {
		s.Log("Cleanup")
		if err := cswitch.CloseSession(cleanupCtx, sessionID, domainIP); err != nil {
			s.Log("Failed to close sessionID: ", err)
		}
	}()

	numTrial := s.Param().(int)
	for i := 0; i < numTrial; i++ {
		if numTrial > 1 {
			s.Logf("Trial %d/%d", i+1, numTrial)
		}
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
			s.Fatal("Failed to enable c-switch port: ", err)
		}

		connected, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort)
		if !connected {
			s.Fatal("Failed to enumerate the TBT device: ", err)
		}

		txSpeed, err := cswitch.TxSpeed(ctx, tbtPort)
		if err != nil {
			s.Fatal("Failed to get the txSpeed: ", err)
		}

		if strings.TrimSpace(txSpeed) != tbtVal["tx_speed"].(string) {
			s.Fatalf("Failed to verify the tx speed, got %s, want %s", txSpeed, tbtVal["tx_speed"].(string))
		}

		rxSpeed, err := cswitch.RxSpeed(ctx, tbtPort)
		if err != nil {
			s.Fatal("Failed to get the rxSpeed: ", err)
		}

		if strings.TrimSpace(rxSpeed) != tbtVal["rx_speed"].(string) {
			s.Fatalf("Failed to verify the rx speed, got %s, want %s", rxSpeed, tbtVal["rx_speed"].(string))
		}

		nvmVersion, err := cswitch.NvmVersion(ctx, tbtPort)
		if err != nil {
			s.Fatal("Failed to get the nvmVersion: ", err)
		}

		if strings.TrimSpace(nvmVersion) != tbtVal["nvme_version"].(string) {
			s.Fatalf("Failed to verify the nvme version, got %s, want %s", nvmVersion, tbtVal["nvme_version"].(string))
		}

		tbtGeneration, err := cswitch.Generation(ctx, tbtPort)
		if err != nil {
			s.Fatal("Failed to get the tbtGeneration: ", err)
		}
		if strings.TrimSpace(tbtGeneration) != tbtVal["generation"].(string) {
			s.Fatalf("Failed to verify the generation, got %s, want %s", tbtGeneration, tbtVal["generation"].(string))
		}

		cSwitchOFF := "0"
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}

		isConnected, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort)
		if isConnected {
			s.Fatal("Failed to disconnect the TBT device: ", err)
		}
	}
}
