// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
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
			{
				Name: "manual",
				Val:  false,
			},
			// For automated testing.
			{
				Name:              "smoke",
				ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
				Val:               true,
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
	present, err := checkPortsForTBTPartner(ctx, d)
	if err != nil {
		s.Log("Couldn't find TBT device from PD identity: ", err)
		return
	}

	// Return early for smoke testing (CQ).
	if smoke := s.Param().(bool); smoke {
		return
	}

	if !present {
		s.Fatal("No TBT device connected to DUT")
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

	found := ""
	for _, device := range strings.Split(string(out), "\n") {
		if device == "" {
			continue
		}

		if device == "domain0" || device == "0-0" {
			continue
		}

		// Check for retimers.
		// They are of the form "0-0:1.1" or "0-0:3.1".
		if matched, err := regexp.MatchString(`[\d\-\:]+\.\d`, device); err != nil {
			return errors.Wrap(err, "couldn't execute retimer regexp")
		} else if matched {
			continue
		}

		found = device
		break
	}

	if expected && found == "" {
		return errors.New("no TBT device found")
	} else if !expected && found != "" {
		return errors.Errorf("TBT device found: %s", found)
	}

	return nil
}

// checkPortsForTBTPartner checks whether the device has a connected Thunderbolt device.
// We use the 'ectool typecdiscovery' command to accomplish this.
//
// This functions returns:
// - Whether a TBT device is connected to the DUT.
// - The error value if the command didn't run, else nil.
func checkPortsForTBTPartner(ctx context.Context, d *dut.DUT) (bool, error) {
	for i := 0; i < maxTypeCPorts; i++ {
		out, err := d.Command("ectool", "typecdiscovery", strconv.Itoa(i), "0").CombinedOutput(ctx)
		if err != nil {
			// If we get an invalid param error, that means there are no more ports left.
			// In that case, we shouldn't return an error, but should return false.
			//
			// TODO(pmalani): Determine how many ports a device supports, instead of
			// relying on INVALID_PARAM.
			if bytes.Contains(out, []byte("INVALID_PARAM")) {
				return false, nil
			}

			return false, errors.Wrap(err, "failed to run ectool command")
		}

		// Look for a TBT SVID in the output. If one doesn't exist, return false.
		if bytes.Contains(out, []byte("SVID 0x8087")) {
			return true, nil
		}
	}

	return false, nil
}
