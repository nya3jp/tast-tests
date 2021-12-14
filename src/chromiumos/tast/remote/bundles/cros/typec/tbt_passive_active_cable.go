// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/typec"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type tbtCableTestParams struct {
	cableType string
	connector string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TBTPassiveActiveCable,
		Desc:         "Verifies connected thunderbolt cable type is passive cable or active cable",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.typec.Service"},
		Attr:         []string{"group:typec"},
		Data:         []string{"test_config.json", "testcert.p12"},
		Vars:         []string{"servo", "typec.cSwitchPort", "typec.dutTbtPort", "typec.domainIP"},
		// TODO(b/207569436): Define hardware dependency and get rid of hard-coding the models.
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel", "redrix", "brya")),
		Params: []testing.Param{{
			Name: "tbt_active",
			Val: tbtCableTestParams{
				cableType: "Active",
				connector: "TBT",
			},
		}, {
			Name: "tbt_passive",
			Val: tbtCableTestParams{
				cableType: "Passive",
				connector: "TBT",
			},
		}, {
			Name: "usb4_active",
			Val: tbtCableTestParams{
				cableType: "Active",
				connector: "USB4",
			},
		}},
	})
}

// TBTPassiveActiveCable checks connected thunderbolt cable is active cable or passive cable.
// Here, "TBT" means Thunderbolt.
func TBTPassiveActiveCable(ctx context.Context, s *testing.State) {
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

	servoSpec, _ := s.Var("servo")
	dut := s.DUT()
	testOpt := s.Param().(tbtCableTestParams)

	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	// Connect to gRPC server
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// Send key file to DUT.
	keyPath := filepath.Join("/tmp", "testcert.p12")
	defer func(ctx context.Context) {
		if err := dut.Conn().CommandContext(ctx, "rm", keyPath).Run(); err != nil {
			s.Errorf("Failed to remove %q file: %v", keyPath, err)
		}
	}(cleanupCtx)

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
		s.Fatalf("Failed to open %v file: %v", testConfig, err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for TBT/USB4 config data.
	connectorVal, ok := data[testOpt.connector].(map[string]interface{})
	if !ok {
		s.Fatalf("Failed to found %q config data in JSON file", testOpt.connector)
	}

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create session: ", err)
	}

	defer func() {
		s.Log("Performing cleanup")
		if err := cswitch.CloseSession(cleanupCtx, sessionID, domainIP); err != nil {
			s.Log("Failed to close session: ", err)
		}
	}()

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	connected, err := isDeviceEnumerated(ctx, dut, connectorVal["device_name"].(string), dutPort)
	if !connected {
		s.Fatal("Failed to enumerate the TBT device: ", err)
	}

	portNum, err := cableConnectedPort(ctx, dut, testOpt.connector)
	if err != nil {
		s.Fatalf("Failed to get %q connected port number: %v", testOpt.connector, err)
	}

	pdCableCommand := fmt.Sprintf("pdcable %s", portNum)
	ecPatterns := []string{"Cable Type: " + testOpt.cableType}
	out, err := pxy.Servo().RunECCommandGetOutput(ctx, pdCableCommand, ecPatterns)
	if err != nil {
		s.Fatal("Failed to run EC command: ", err)
	}

	expectedOut := "Cable Type: " + testOpt.cableType
	actualOut := out[0][0]
	if actualOut != expectedOut {
		s.Fatalf("Unexpected cable type. Want %q; got %q", expectedOut, actualOut)
	}
}

// isDeviceEnumerated validates device enumeration in DUT.
// device holds the device name of connected TBT/USB4 device.
// port holds the TBT/USB4 port ID in DUT.
func isDeviceEnumerated(ctx context.Context, dut *dut.DUT, device, port string) (bool, error) {
	deviceNameFile := fmt.Sprintf("/sys/bus/thunderbolt/devices/%s/device_name", port)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := linuxssh.ReadFile(ctx, dut.Conn(), deviceNameFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read %q file", deviceNameFile)
		}

		if strings.TrimSpace(string(out)) != device {
			return errors.New("Device enumeration failed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		return false, err
	}
	return true, nil
}

// cableConnectedPort on success will returns Active/Passive cable connected port number.
func cableConnectedPort(ctx context.Context, dut *dut.DUT, connector string) (string, error) {
	out, err := dut.Conn().CommandContext(ctx, "ectool", "usbpdmuxinfo").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to execute ectool usbpdmuxinfo command")
	}
	portRe := regexp.MustCompile(fmt.Sprintf(`Port.([0-9]):.*(%s=1)`, connector))
	portNum := portRe.FindStringSubmatch(string(out))
	if portNum[1] == "" {
		return "", errors.New("failed to get port number from usbpdmuxinfo")
	}
	return portNum[1], nil
}
