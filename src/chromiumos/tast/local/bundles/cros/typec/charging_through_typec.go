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
	"chromiumos/tast/local/cswitch"
	pow "chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type chargeTypecTestParams struct {
	connector string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChargingThroughTypec,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checking device charging status after USB4/TBT dock hot plug-unplug",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"test_config.json"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel", "redrix", "brya")),
		Params: []testing.Param{{
			Name: "usb4",
			Val: chargeTypecTestParams{
				connector: "USB4",
			},
		}, {
			Name: "tbt_dock",
			Val: chargeTypecTestParams{
				connector: "TBT",
			},
		}},
	})
}

// ChargingThroughTypec performs the following:
// - Pre-requisite : Remove the charger from DUT and make sure USB4/TBT dock is connected.
// - Hot plug USB4/TBT Device into DUT with help of cswitch.
// - Performs USB4/TBT Device enumeration check.
// - Check DUT charging state.
// - Unplug USB4/TBT device from DUT and validates USB4/TBT device removal detection.
// - Check DUT charging state.
// This test requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug) ----> USB4/TBT dock.

// ChargingThroughTypec func validates DUT charging state on USB4/TBT hot plug-unplug.
func ChargingThroughTypec(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(chargeTypecTestParams)
	// Config file which contains expected values of USB4/TBT parameters.
	const testConfig = "test_config.json"
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

	// Read json config file.
	jsonData, err := ioutil.ReadFile(s.DataPath(testConfig))
	if err != nil {
		s.Fatalf("Failed to open %v file : %v", testConfig, err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for USB4/TBT config data.
	connectorVal, ok := data[testOpt.connector].(map[string]interface{})
	if !ok {
		s.Fatalf("Failed to find %s config data in JSON file", testOpt.connector)
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

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	if _, err := cswitch.IsDeviceEnumerated(ctx, connectorVal["device_name"].(string), tbtPort); err != nil {
		s.Fatal("Failed to enumerate the TBT device: ", err)
	}

	tbtGeneration, err := cswitch.Generation(ctx, tbtPort)
	if err != nil {
		s.Fatal("Failed to get the tbtGeneration: ", err)
	}
	if strings.TrimSpace(tbtGeneration) != connectorVal["generation"].(string) {
		s.Fatalf("Failed to verify the generation, got %s, want %s", tbtGeneration, connectorVal["generation"].(string))
	}

	// Verifying battery charging status after USB4/TBT hot-plug.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		status, err := pow.GetStatus(ctx)
		if err != nil {
			s.Fatal("Failed to get power status: ", err)
		}
		if status.BatteryDischarging {
			return errors.Errorf("failed to charge after %s hot-plug", testOpt.connector)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Battery status : ", err)
	}

	cSwitchOFF := "0"
	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	if _, err := cswitch.IsDeviceEnumerated(ctx, connectorVal["device_name"].(string), tbtPort); err == nil {
		s.Fatal("Failed to disconnect the TBT device: ", err)
	}

	status, err := pow.GetStatus(ctx)
	if err != nil {
		s.Fatal("Failed to get power status: ", err)
	}
	// Verifying battery discharging after USB4/TBT hot-unplug.
	if !status.BatteryDischarging {
		s.Fatalf("Battery failed to discharge after %s hot-unplug", testOpt.connector)
	}
}
