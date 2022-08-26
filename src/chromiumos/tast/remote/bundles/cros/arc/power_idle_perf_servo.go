// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PowerIdlePerfServo,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Uses a servo to measure the battery drain of an idle system running ARC",
		Contacts: []string{
			"cwd@google.com",
		},
		Attr:         []string{"group:crosbolt"},
		Fixture:      fixture.NormalMode,
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.arc.PowerPerfService"},
		Timeout:      45 * time.Minute,

		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "vm_doze",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func measurePower(ctx context.Context, h *firmware.Helper, duration time.Duration) error {
	output, err := h.ServoProxy.OutputCommand(ctx, false, "dut-power",
		"--ina-rate", "0",
		"--vbat-rate", "0.1",
		"-t", strconv.Itoa(int(duration.Seconds())),
		"--save-raw-data",
	)
	if err != nil {
		return errors.Wrap(err, "failed to run dut-power")
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get OutDir")
	}

	for _, line := range strings.Split(string(output), "\n") {
		getFile := false
		if strings.HasSuffix(line, "/raw_data/ec_time.txt") {
			getFile = true
		} else if strings.HasSuffix(line, "/raw_data/ec_ppvar_vbat.txt") {
			getFile = true
		} else if strings.HasSuffix(line, "/raw_data/ec_Sample_msecs.txt") {
			getFile = true
		} else if strings.HasSuffix(line, "/raw_data/ec_timeline.txt") {
			getFile = true
		}

		if getFile {
			if err := h.ServoProxy.GetFile(ctx, false, line, filepath.Join(outDir, filepath.Base(line))); err != nil {
				return errors.Wrap(err, "failed to get output file from dut-power")
			}
		}
	}

	testing.ContextLog(ctx, "Measurement done: ", string(output))

	return nil
}

func PowerIdlePerfServo(ctx context.Context, s *testing.State) {
	s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewPowerPerfServiceClient(cl.Conn)
	if _, err := service.PowerSetup(ctx, &arc.PowerSetupRequest{
		Duration: durationpb.New(30 * time.Minute),
		ArcDoze:  strings.Contains(s.TestName(), "doze"),
	}); err != nil {
		s.Fatal("Failed to set up DUT for power test: ", err)
	}
	s.Log("Power setup complete")

	s.Log("Starting power measurements")
	if err := measurePower(ctx, h, 10*time.Minute); err != nil {
		s.Error("Failed to measure power: ", err)
	}

	s.Log("Power measurement complete")
	if _, err := service.PowerCleanup(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to clean up DUT after power test: ", err)
	}
	s.Log("Power cleanup complete")
}
