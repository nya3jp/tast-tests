// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// The maximum number of USB Type C ports that a Chromebook supports.
const maxTypeCPorts = 8

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModeReboot,
		Desc:         "Demonstrates USB Type C mode selection after reboot",
		Contacts:     []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		SoftwareDeps: []string{"tpm2", "reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Params: []testing.Param{
			// For running manually.
			{},
			// For automated testing.
			{
				Name:              "test",
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
			}},
	})
}

// ModeReboot does the following:
// - Log in on a DUT.
// - Validate TBT alt mode is working correctly.
// - Reboot the system.
// - Validate that we are *not* in TBT alt mode and that USB+DP mode is working correctly.
//
// This test requires the following H/W topology to run.
//
//        DUT ------> Thunderbolt3 (>= Titan Ridge) dock -----> DP monitor.
//      (USB4)
//
func ModeReboot(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	// Connect to gRPC server
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// Check if a TBT device is connected. If one isn't, we should skip
	// execution.
	// Check each port successively. If a port returns an error, that means
	// we are out of ports.
	// This check is for test executions which take place on
	// CQ (where TBT peripherals aren't connected).
	for i := 0; i < maxTypeCPorts; i++ {
		if present, err := checkPortForTBTPartner(ctx, d, i); err != nil {
			s.Log("Couldn't find TBT device from PD identity: ", err)
			return
		} else if present {
			s.Log("Found a TBT device, proceeding with test")
			break
		}
	}

	// Login to Chrome.
	client := security.NewBootLockboxServiceClient(cl.Conn)
	_, err = client.NewChromeLogin(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	s.Log("Verifying that a TBT device is enumerated")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		return checkTBTDevice(ctx, d, true)
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed TBT enumeration after login: ", err)
	}

	s.Log("Rebooting the DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	if !d.Connected(ctx) {
		s.Fatal("Failed to connect to DUT post reboot")
	}

	if err = testing.Poll(ctx, func(ctx context.Context) error {
		return checkTBTDevice(ctx, d, false)
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed TBT non-enumeration after reboot: ", err)
	}

	// TODO(b/179338646): Check that there is a connected DP monitor.
}

// checkTBTDevice is a helper function which checks for TBT device connection to a DUT.
// |expected| specifies whether we want to check for the presence of a TBT device (true) or the
// absence of one (false).
func checkTBTDevice(ctx context.Context, d *dut.DUT, expected bool) error {
	out, err := d.Command("ls", "/sys/bus/thunderbolt/devices").Output(ctx)
	if err != nil {
		return errors.Wrap(err, "could not run ls command on DUT")
	}

	found := false
	for _, device := range strings.Split(string(out), "\n") {
		if device == "" {
			continue
		}

		if device != "domain0" && device != "0-0" {
			found = true
			break
		}
	}

	if expected && !found {
		return errors.New("no TBT device found")
	} else if !expected && found {
		return errors.New("TBT device found")
	}

	return nil
}

// checkPortForTBTPartner checks whether the device has a connected Thunderbolt device.
// We use the 'ectool typecdiscovery' command to accomplish this.
// If |port| is invalid, the ectool command should return an INVALID_PARAM error.
//
// This functions returns:
// - Whether a TBT device is present at a given port.
// - The error value if the command didn't run, else nil.
func checkPortForTBTPartner(ctx context.Context, d *dut.DUT, port int) (bool, error) {
	out, err := d.Command("ectool", "typecdiscovery", strconv.Itoa(port), "0").Output(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to run ectool command")
	}

	// Look for a TBT SVID in the output. If one doesn't exist, return false.
	return regexp.MatchString(`SVID 0x8087`, string(out))
}
