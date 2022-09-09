// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/power"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemotePowerIdlePerf,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Uses a servo to measure the battery drain of an idle system running ARC",
		Contacts: []string{
			"cwd@google.com",
		},
		Attr:         []string{"group:crosbolt"},
		Fixture:      fixture.NormalMode,
		SoftwareDeps: []string{"chrome", "arc"},
		ServiceDeps:  []string{"tast.cros.arc.PowerPerfService"},
		Timeout:      60 * time.Minute,
	})
}

func RemotePowerIdlePerf(ctx context.Context, s *testing.State) {
	s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	pow := power.NewDutPowerContext(ctx, h)
	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewPowerPerfServiceClient(cl.Conn)
	if _, err := service.PowerSetup(ctx, &arc.PowerSetupRequest{
		Duration: durationpb.New(70 * time.Minute),
	}); err != nil {
		s.Fatal("Failed to set up DUT for power test: ", err)
	}
	s.Log("Power setup complete")

	s.Log("Starting power measurements")

	p := perf.NewValues()
	results, err := pow.Measure(25 * time.Minute)
	if err == nil {
		if ppvar, ok := results.GetMean("ppvar_vbat"); ok {
			p.Set(perf.Metric{
				Name:      "ppvar_vbat",
				Unit:      "mW",
				Direction: perf.SmallerIsBetter,
			}, float64(ppvar))
		}
		if mean, ok := results.GetMean("ppdut5"); ok {
			p.Set(perf.Metric{
				Name:      "ppdut5",
				Unit:      "W",
				Direction: perf.SmallerIsBetter,
			}, float64(mean))
		}
		if mean, ok := results.GetMean("ppchg5"); ok {
			p.Set(perf.Metric{
				Name:      "ppchg5",
				Unit:      "W",
				Direction: perf.SmallerIsBetter,
			}, float64(mean))
		}
	} else {
		s.Error("Failed to measure power: ", err)
	}

	s.Log("Power measurement complete")
	values, err := service.PowerCleanup(ctx, &emptypb.Empty{})
	if err != nil {
		s.Fatal("Failed to clean up DUT after power test: ", err)
	}
	s.Log("Power cleanup complete")
	p.Merge(perf.NewValuesFromProto(values))
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf values: ", err)
	}

}
