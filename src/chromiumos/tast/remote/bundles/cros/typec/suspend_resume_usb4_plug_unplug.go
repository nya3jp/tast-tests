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
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/usbutils"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/remote/bundles/cros/typec/typecutils"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/typec"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuspendResumeUSB4PlugUnplug,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies USB4 docking station using 40G passive cable: Insert before suspend, unplug during suspend, insert back in suspend, then resume",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:typec"},
		ServiceDeps:  []string{"tast.cros.typec.Service"},
		Data:         []string{"test_config.json", "testcert.p12"},
		VarDeps:      []string{"servo", "typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP"},
		Timeout:      15 * time.Minute,
	})
}

func SuspendResumeUSB4PlugUnplug(ctx context.Context, s *testing.State) {
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
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Error("Failed to power on DUT in cleanup: ", err)
			}
		}
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}

		if err := cswitch.CloseSession(ctx, sessionID, domainIP); err != nil {
			s.Error("Failed to close sessionID: ", err)
		}
	}(cleanupCtx)

	iterationValue := 10
	for i := 1; i <= iterationValue; i++ {

		s.Logf("Iteration: %d/%d", i, iterationValue)
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

		connected, err := typecutils.IsDeviceEnumerated(ctx, dut, usb4Val["device_name"].(string), dutPort)
		if err != nil && !connected {
			s.Fatal("Failed to enumerate the TBT device: ", err)
		}

		var usbDevicesList []usbutils.USBDevice
		usbDeviceClassName := "Mass Storage"
		usbSpeed := "5000M"
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			usbDevicesList, err = usbutils.ListDevicesInfo(ctx, dut)
			if err != nil {
				return errors.Wrap(err, "failed to get USB devices list")
			}
			got := usbutils.NumberOfUSBDevicesConnected(usbDevicesList, usbDeviceClassName, usbSpeed)
			if want := 1; got != want {
				return errors.Errorf("unexpected number of USB devices connected: got %d, want %d", got, want)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Failed to wait for device list info: ", err)
		}

		slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
		if err != nil {
			s.Fatal("Failed to get SLP counter and C10 package values before suspend-resume: ", err)
		}

		suspendCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := dut.Conn().CommandContext(suspendCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			s.Fatal("Failed to power off DUT: ", err)
		}

		shCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(shCtx); err != nil {
			s.Fatal("Failed to wait for unreachable: ", err)
		}

		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}

		waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()
		if err := dut.WaitConnect(waitCtx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}

		slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
		if err != nil {
			s.Fatal("Failed to get SLP counter and C10 package values after suspend-resume: ", err)
		}

		if slpOpSetPre == slpOpSetPost {
			s.Fatalf("Failed: SLP counter value %q should be different from the one before suspend %q", slpOpSetPost, slpOpSetPre)
		}

		if slpOpSetPost == 0 {
			s.Fatal("Failed SLP counter value must be non-zero, got: ", slpOpSetPost)
		}

		if pkgOpSetPre == pkgOpSetPost {
			s.Fatalf("Failed: Package C10 value %q must be different from the one before suspend %q", pkgOpSetPost, pkgOpSetPre)
		}

		if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
			s.Fatal("Failed: Package C10 should be non-zero")
		}

		if err := dut.Conn().CommandContext(suspendCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			s.Fatal("Failed to power off DUT: ", err)
		}

	}
}
