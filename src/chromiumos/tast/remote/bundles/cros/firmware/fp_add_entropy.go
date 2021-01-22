// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpAddEntropy,
		Desc: "Validate adding entropy only succeeds when running RO",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService"},
		Vars:         []string{"servo"},
	})
}

func FpAddEntropy(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if err := fingerprint.InitializeKnownState(ctx, d, s.OutDir()); err != nil {
		s.Fatal("Initialization failed: ", err)
	}

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer cleanup(ctx, s, d, pxy)

	// TODO(yichengli): Check the FPMCU is running expected firmware version.

	if err := fingerprint.InitializeHWAndSWWriteProtect(ctx, d, pxy, true, true); err != nil {
		s.Fatal("Initialization failed: ", err)
	}

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	upstartService := platform.NewUpstartServiceClient(cl.Conn)

	// Waiting for biod to be running in case biod wants to set entropy.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		_, err := upstartService.CheckJob(ctx, &platform.CheckJobRequest{JobName: "biod"})
		return err
	}, &testing.PollOptions{Timeout: fingerprint.WaitForBiodToStartTimeout})

	if err != nil {
		s.Fatal("Timed out waiting for biod to start: ", err)
	}

	firmwareCopy, err := fingerprint.RunningFirmwareCopy(ctx, d)
	if err != nil {
		s.Fatal("Failed to query running firmware copy: ", err)
	}
	if firmwareCopy != "RW" {
		s.Fatal("Not running RW firmware")
	}

	if err := fingerprint.CheckRollbackSetToInitialValue(ctx, d); err != nil {
		s.Fatal("Failed to validate rollback state: ", err)
	}

	testing.ContextLog(ctx, "Adding entropy should fail when running RW")
	if err := fingerprint.AddEntropy(ctx, d, false); err == nil {
		s.Fatal("Adding entropy succeeded when running RW")
	}

	testing.ContextLog(ctx, "Validating rollback didn't change")
	if err := fingerprint.CheckRollbackSetToInitialValue(ctx, d); err != nil {
		s.Fatal("Failed to validate rollback state: ", err)
	}

	testing.ContextLog(ctx, "Adding entropy from RO should succeed")
	if err := fingerprint.RebootFpmcu(ctx, d, "RO"); err != nil {
		s.Fatal("Failed to reboot to RO: ", err)
	}
	_ = fingerprint.AddEntropy(ctx, d, false)
	testing.ContextLog(ctx, "Validating Block ID changes, but nothing else")
	if err := fingerprint.CheckRollbackState(ctx, d, 2, 0, 0); err != nil {
		s.Fatal("Unexpected rollback state: ", err)
	}

	testing.ContextLog(ctx, "Adding entropy with reset (double write) from RO should succeed")
	if err := fingerprint.RebootFpmcu(ctx, d, "RO"); err != nil {
		s.Fatal("Failed to reboot to RO: ", err)
	}
	_ = fingerprint.AddEntropy(ctx, d, true)
	testing.ContextLog(ctx, "Validating Block ID increases by 2, but nothing else")
	if err := fingerprint.CheckRollbackState(ctx, d, 4, 0, 0); err != nil {
		s.Fatal("Unexpected rollback state: ", err)
	}

	testing.ContextLog(ctx, "Switching back to RW")
	if err := fingerprint.RebootFpmcu(ctx, d, "RW"); err != nil {
		s.Fatal("Failed to reboot to RW: ", err)
	}
	testing.ContextLog(ctx, "Validating nothing changed")
	if err := fingerprint.CheckRollbackState(ctx, d, 4, 0, 0); err != nil {
		s.Fatal("Unexpected rollback state: ", err)
	}
}

// cleanup restores the original fingerprint firmware.
func cleanup(ctx context.Context, s *testing.State, d *dut.DUT, pxy *servo.Proxy) {
	testing.ContextLog(ctx, "Starting cleanup at the end of test")
	defer pxy.Close(ctx)
	if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, "force_off"); err != nil {
		s.Fatal("Failed to disable HW write protect: ", err)
	}

	if err := fingerprint.FlashFirmware(ctx, d); err != nil {
		s.Error("Failed to flash original FP firmware: ", err)
	}

	if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, "force_on"); err != nil {
		s.Fatal("Failed to enable HW write protect: ", err)
	}
}
