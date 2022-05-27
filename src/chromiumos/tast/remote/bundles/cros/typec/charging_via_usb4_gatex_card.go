// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/remote/bundles/cros/typec/setup"
	"chromiumos/tast/remote/bundles/cros/typec/typecutils"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/typec"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChargingViaUsb4GatexCard,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies DUT charging through USB4 Gatkex creek Card using 40G passive cable",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:typec"},
		ServiceDeps:  []string{"tast.cros.typec.Service"},
		Data:         []string{"test_config.json", "testcert.p12"},
		VarDeps:      []string{"servo", "typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery(), setup.ThunderboltSupportedDevices()),
		Timeout:      5 * time.Minute,
	})
}

func ChargingViaUsb4GatexCard(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	// Config file which contains expected values of USB4 parameters.
	const testConfig = "test_config.json"
	// TBT port ID in the DUT.
	dutPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	servoSpec := s.RequiredVar("servo")

	dut := s.DUT()

	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	// Connect to gRPC server.
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// Send key file to DUT.
	keyPath := filepath.Join("/tmp", "testcert.p12")
	defer dut.Conn().CommandContext(cleanupCtx, "rm", keyPath).Run()

	testcertKeyPath := map[string]string{s.DataPath("testcert.p12"): keyPath}
	if _, err := linuxssh.PutFiles(ctx, dut.Conn(), testcertKeyPath, linuxssh.DereferenceSymlinks); err != nil {
		s.Fatalf("Failed to send data to remote data path %v: %v", keyPath, err)
	}

	// Login to Chrome.
	client := typec.NewServiceClient(cl.Conn)
	_, err = client.NewChromeLoginWithPeripheralDataAccess(ctx, &typec.KeyPath{Path: keyPath})
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

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
		s.Fatal("Failed to find USB4 config data in JSON file")
	}

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create sessionID: ", err)
	}

	cSwitchOFF := "0"
	defer func(ctx context.Context) {
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}
		if err := cswitch.CloseSession(ctx, sessionID, domainIP); err != nil {
			s.Error("Failed to close sessionID: ", err)
		}
	}(cleanupCtx)

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	var portNum string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		portNum, err = typecutils.CableConnectedPortNumber(ctx, dut, "USB4")
		if err != nil {
			return errors.Wrap(err, "failed to get USB4 connected port number")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		s.Fatal("Failed to get cable connected port number: ", err)
	}

	pdCableCommand := fmt.Sprintf("pdcable %s", portNum)
	ecPatterns := []string{"Cable Type: " + "Passive"}
	out, err := pxy.Servo().RunECCommandGetOutput(ctx, pdCableCommand, ecPatterns)
	if err != nil {
		s.Fatal("Failed to run EC command: ", err)
	}

	expectedOut := "Cable Type: " + "Passive"
	actualOut := out[0][0]
	if actualOut != expectedOut {
		s.Fatalf("Unexpected cable type. Want %q; got %q", expectedOut, actualOut)
	}

	if err := typecutils.CheckUSBPdMuxinfo(ctx, dut, "USB4=1"); err != nil {
		s.Fatal("Failed to verify dmesg logs: ", err)
	}

	connected, err := typecutils.IsDeviceEnumerated(ctx, dut, usb4Val["device_name"].(string), dutPort)
	if err != nil && !connected {
		s.Fatal("Failed to enumerate the TBT device: ", err)
	}

	if err := verifyPowerSupplyInfo(ctx, dut, true); err != nil {
		s.Fatal("Failed to verify power supply info: ", err)
	}

	// Check battery info with power_supply_info command.
	if err := verifyPowerSupplyInfo(ctx, dut, true); err != nil {
		s.Fatal("Failed to verify power supply info: ", err)
	}

	// Check battery info with ectool battery flag.
	if err := verifyEctoolBatteryStatus(ctx, dut, true); err != nil {
		s.Fatal("Failed to verify battery status using ectool battery flag: ", err)
	}
	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}
	// Check battery info with power_supply_info command.
	if err := verifyPowerSupplyInfo(ctx, dut, false); err != nil {
		s.Fatal("Failed to verify power supply info: ", err)
	}

	// Check battery info with ectool battery flag.
	if err := verifyEctoolBatteryStatus(ctx, dut, false); err != nil {
		s.Fatal("Failed to verify battery status using ectool battery flag: ", err)
	}
}

// verifyPowerSupplyInfo verifies whether DUT is charging or not iusing power_supply_info command.
func verifyPowerSupplyInfo(ctx context.Context, dut *dut.DUT, supply bool) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := dut.Conn().CommandContext(ctx, "power_supply_info").Output()
		if err != nil {
			return errors.Wrap(err, "failed to get power supply info")
		}
		if supply == true {
			chargeStateRe := regexp.MustCompile(`state.*Charging`)
			if !chargeStateRe.MatchString(string(out)) {
				return errors.New("unexpected power_supply_info state: got discharging, want charging")
			}
		} else if supply == false {
			dischargeStateRe := regexp.MustCompile(`state.*Discharging`)
			if !dischargeStateRe.MatchString(string(out)) {
				return errors.New("unexpected power_supply_info state: got charging, want discharging")
			}
		}
		return nil

	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 1 * time.Second}); err != nil {
	}
	return nil
}

// verifyEctoolBatteryStatus checks ectool battery flag is discharging or not.
func verifyEctoolBatteryStatus(ctx context.Context, dut *dut.DUT, charging bool) error {
	out, err := dut.Conn().CommandContext(ctx, "ectool", "battery").Output()
	if err != nil {
		return errors.Wrap(err, "failed to get ectool battery info")
	}
	if charging == true {
		chargeFlagRe := regexp.MustCompile(`Flags.*AC_PRESENT.*BATT_PRESENT.*CHARGING`)
		if !chargeFlagRe.MatchString(string(out)) {
			return errors.New("unexpected battery flag: got discharging, want charging")
		}
	} else if charging == false {
		dischargeFlagRe := regexp.MustCompile(`Flags.*BATT_PRESENT.*DISCHARGING`)
		if !dischargeFlagRe.MatchString(string(out)) {
			return errors.New("unexpected battery flag: got charging, want discharging")
		}
	}
	return nil
}
