// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

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
	s.Log("Configuring DUT for power test")
	if _, err := service.Setup(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to set up DUT for power test: ", err)
	}
	defer func(ctx context.Context) {
		s.Log("Restoring DUT configuration after power test")
		if _, err := service.Cleanup(ctx, &emptypb.Empty{}); err != nil {
			s.Fatal("Failed to clean up DUT after power test: ", err)
		}
	}(ctx)

	if _, err := service.StartMeasurement(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to start power measurement: ", err)
	}

	s.Log("Starting power measurements")
	p := perf.NewValues()
	results, err := pow.MeasureAllSummaries( /*25 */ time.Minute)
	if err != nil {
		s.Fatal("Failed to measure power: ", err)
	}

	for summaryName, summary := range results {
		for name, value := range summary {
			if name == "time" || name == "Sample_msecs" || name == "timeline" {
				continue
			}
			p.Set(perf.Metric{
				Name:      summaryName + "_" + name,
				Unit:      "power",
				Direction: perf.SmallerIsBetter,
			}, float64(value))
		}
	}

	values, err := service.StopMeasurement(ctx, &emptypb.Empty{})
	if err != nil {
		s.Fatal("Failed to stop power measurement: ", err)
	}
	p.Merge(perf.NewValuesFromProto(values))

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf values: ", err)
	}
}
