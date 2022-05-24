// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UsbEthernetBehaviourColdReboot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "USB type-C/type-A to Ethernet adapter behaves properly when device cold reboots",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.network.EthernetService"},
		Fixture:      fixture.NormalMode,
		Vars:         []string{"network.iterations"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Name: "typea",
			Val:  regexp.MustCompile(`If 0.*Class=.*480M`),
		}, {
			Name: "typec",
			Val:  regexp.MustCompile(`If 0.*Class=.*5000M`),
		}},
	})
}

func UsbEthernetBehaviourColdReboot(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	expectedUsbType := s.Param().(*regexp.Regexp)

	if err := checkUSBType(ctx, expectedUsbType); err != nil {
		s.Fatal("Failed to verify USB type: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	iteration := 1
	if val, ok := s.Var("network.iterations"); ok {
		var err error
		iteration, err = strconv.Atoi(val)
		if err != nil {
			s.Fatal("Failed to convert provided iterations to an integer: ", err)
		}
	}

	browseOverEthernet := func() {
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		client := network.NewEthernetServiceClient(cl.Conn)

		s.Log("Sleeping for a few seconds before starting a new Chrome")
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to sleep for a few seconds: ", err)
		}

		if _, err := client.New(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}

		request := network.WifiRequest{Enabled: false}
		if _, err := client.SetWifi(ctx, &request); err != nil {
			s.Fatal("Failed to disable Wi-Fi: ", err)
		}

		urlrequest := network.BrowseRequest{Url: "https://www.google.com"}
		if _, err := client.Browse(ctx, &urlrequest); err != nil {
			s.Fatal("Failed to browse: ", err)
		}
	}

	for i := 1; i <= iteration; i++ {
		s.Logf("Iteration: %d/%d", i, iteration)

		browseOverEthernet()

		cmd := h.DUT.Conn().CommandContext(ctx, "/sbin/shutdown", "-h", "now")
		if err := cmd.Start(); err != nil {
			s.Fatal("Failed to shut down DUT: ", err)
		}

		if err := h.WaitForPowerStates(ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "G3"); err != nil {
			s.Fatal("Failed to get G3 powerstate: ", err)
		}

		s.Log("Power DUT back on with short press of the power button")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			s.Fatal("Failed to power on DUT with short press of the power button: ", err)
		}

		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to reconnect to DUT after restarting: ", err)
		}

		browseOverEthernet()
	}
}

// checkUSBType verifies the expected USB version, and returns an error if it does not match.
func checkUSBType(ctx context.Context, usbDetectionRe *regexp.Regexp) error {
	lsusbCMD := "lsusb -t"
	out, err := testexec.CommandContext(ctx, "sh", "-c", lsusbCMD).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to execute lsusb command")
	}

	if !usbDetectionRe.MatchString(string(out)) {
		return errors.New("failed to verify usb version")
	}
	return nil
}
