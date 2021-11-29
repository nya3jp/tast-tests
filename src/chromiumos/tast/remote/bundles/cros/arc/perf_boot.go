// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfBoot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Signs in to DUT and measures Android boot performance metrics",
		Contacts: []string{
			"cywang@chromium.org", // Original author.
			"niwa@chromium.org",   // Tast port author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.arc.PerfBootService"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func PerfBoot(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewPerfBootServiceClient(cl.Conn)

	if _, err := service.WaitUntilCPUCoolDown(ctx, &empty.Empty{}); err != nil {
		s.Fatal("PerfBootService.WaitUntilCPUCoolDown returned an error: ", err)
	}

	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Need to reconnect to the gRPC server after rebooting DUT.
	cl, err = rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service = arc.NewPerfBootServiceClient(cl.Conn)

	res, err := service.GetPerfValues(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("PerfBootService.GetPerfValues returned an error: ", err)
	}
	for _, m := range res.Values {
		if m.Multiple {
			s.Logf("Logcat event entry: tag=%s unit=%s values=%v", m.Name, m.Unit, m.Value)
		} else {
			s.Logf("Logcat event entry: tag=%s unit=%s value=%f", m.Name, m.Unit, m.Value[0])
		}
	}

	p := perf.NewValuesFromProto(res)
	if err = p.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf data: ", err)
	}
}
