// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/remote/firmware/fingerprint"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	waitForBiodToStartTimeout = 30 * time.Second
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
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
		ServiceDeps:  []string{"tast.cros.platform.UpstartService"},
	})
}

func FpSensor(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if err := fingerprint.InitializeKnownState(ctx, d, s.OutDir()); err != nil {
		s.Fatal("Initialization failed: ", err)
	}

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
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
	}, &testing.PollOptions{Timeout: waitForBiodToStartTimeout})

	if err != nil {
		s.Fatal("Timed out waiting for biod to start: ", err)
	}

	fpencstatusCmd := []string{"ectool", "--name=cros_fp", "fpencstatus"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(fpencstatusCmd))
	out, err := d.Command(fpencstatusCmd[0], fpencstatusCmd[1:]...).Output(ctx)

	if err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(fpencstatusCmd), err)
	}
	re := regexp.MustCompile("FPMCU encryption status: 0x[a-f0-9]{7}1(.+)FPTPM_seed_set")
	if !re.MatchString(string(out)) {
		s.Errorf("FPTPM seed is not set; output %q doesn't match regex %q", string(out), re)
	}
}
