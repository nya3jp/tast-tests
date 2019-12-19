// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/local/perf"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PerfBoot,
		Desc: "Signs in to DUT and measures Android boot performance metrics",
		Contacts: []string{
			"cywang@chromium.org", // Original author.
			"niwa@chromium.org",   // Tast port author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome"},
		ServiceDeps:  []string{"tast.cros.arc.PerfBootService"},
		Timeout:      5 * time.Minute,
	})
}

func PerfBoot(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl1, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl1.Close(ctx)

	service := arc.NewPerfBootServiceClient(cl1.Conn)

	if _, err := service.WaitUntilCPUCoolDown(ctx, &empty.Empty{}); err != nil {
		s.Fatal("PerfBootService.WaitUntilCPUCoolDown returned an error: ", err)
	}

	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Need to reconnect to the gRPC server after rebooting DUT.
	cl2, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl2.Close(ctx)

	service = arc.NewPerfBootServiceClient(cl2.Conn)

	res, err := service.GetPerfValues(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("PerfBootService.GetPerfValues returned an error: ", err)
	}

	perfValues := perf.NewValues()

	for _, val := range res.Values {
		s.Logf("Logcat event entry: tag=%s time=%dms", val.Name, val.ValueMs)
		perfValues.Set(perf.Metric{
			Name:      val.Name,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, float64(val.ValueMs))
	}

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}
