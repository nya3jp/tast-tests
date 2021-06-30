// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"

	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpSensor,
		Desc: "Checks that ectool commands for fingerprint sensor behave as expected",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "group:fingerprint-cq", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService", dutfs.ServiceName},
		Vars:         []string{"servo"},
	})
}

func FpSensor(ctx context.Context, s *testing.State) {
	d := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	dutfsClient := dutfs.NewClient(cl.Conn)

	fpBoard, err := fingerprint.Board(ctx, d)
	if err != nil {
		s.Fatal("Failed to get fingerprint board: ", err)
	}

	buildFWFile, err := fingerprint.FirmwarePath(ctx, d, fpBoard)
	if err != nil {
		s.Fatal("Failed to get build firmware file path: ", err)
	}

	needsReboot, err := fingerprint.NeedsRebootAfterFlashing(ctx, d)
	if err != nil {
		s.Fatal("Failed to determine whether reboot is needed: ", err)
	}

	if err := fingerprint.InitializeKnownState(ctx, d, dutfsClient, s.OutDir(), pxy, fpBoard, buildFWFile, needsReboot); err != nil {
		s.Fatal("Initialization failed: ", err)
	}

	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	upstartService := platform.NewUpstartServiceClient(cl.Conn)

	// The seed is only set after bio_crypto_init runs. biod will only start after
	// bio_crypto_init runs, so waiting for biod to be running is sufficient.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		_, err := upstartService.CheckJob(ctx, &platform.CheckJobRequest{JobName: "biod"})
		return err
	}, &testing.PollOptions{Timeout: fingerprint.WaitForBiodToStartTimeout})

	if err != nil {
		s.Fatal("Timed out waiting for biod to start: ", err)
	}

	out, err := fingerprint.EctoolCommand(ctx, d, "fpencstatus").Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get encryption status: ", err)
	}

	re := regexp.MustCompile("FPMCU encryption status: 0x[a-f0-9]{7}1(.+)FPTPM_seed_set")
	if !re.MatchString(string(out)) {
		s.Errorf("FPTPM seed is not set; output %q doesn't match regex %q", string(out), re)
	}
}
