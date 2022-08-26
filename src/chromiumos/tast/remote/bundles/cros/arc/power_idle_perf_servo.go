// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/power"
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
		}},
	})
}

func PowerIdlePerfServo(ctx context.Context, s *testing.State) {
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
	if _, err := service.PowerSetup(ctx, &arc.PowerSetupRequest{Duration: durationpb.New(30 * time.Minute)}); err != nil {
		s.Fatal("Failed to set up DUT for power test: ", err)
	}
	s.Log("Power setup complete")

	powerContext := power.NewDutPowerContext(ctx, h)

	s.Log("Starting power measurements")
	results, err := powerContext.Measure(time.Minute)
	if err != nil {
		s.Fatalf("Failed to measure power: %s", err)
	}

	s.Log("Power measurement complete")
	if _, err := service.PowerCleanup(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to clean up DUT after power test: ", err)
	}
	s.Log("Power cleanup complete")

	s.Log(results)
}
