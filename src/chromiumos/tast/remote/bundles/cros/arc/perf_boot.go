// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/perf/perfpb"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

var (
	defaultIterations = 10 // The number of boot iterations. Can be overridden by var "arc.PerfBoot.iterations".
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
		Vars:         []string{"arc.PerfBoot.iterations"},
		Timeout:      20 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func perfBootOnce(ctx context.Context, s *testing.State, i, iterations int) *perfpb.Values {
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

	// Save raw data for this iteration.
	savedRaw := filepath.Join(s.OutDir(), fmt.Sprintf("raw.%03d", i+1))
	if err = os.Mkdir(savedRaw, 0755); err != nil {
		s.Fatalf("Failed to create path %s", savedRaw)
	}

	p := perf.NewValuesFromProto(res)
	if err = p.Save(savedRaw); err != nil {
		s.Fatal("Failed to save perf raw data: ", err)
	}

	return res
}

func PerfBoot(ctx context.Context, s *testing.State) {
	iterations := defaultIterations
	if iter, ok := s.Var("arc.PerfBoot.iterations"); ok {
		if i, err := strconv.Atoi(iter); err == nil {
			iterations = i
		} else {
			// User might want to override the default value of iterations but passed a malformed value. Fail the test to inform the user.
			s.Fatal("Invalid arc.PerfBoot.iterations value: ", iter)
		}
	}

	pv := perf.NewValues()
	singleMetrics := make(map[perf.Metric][]float64)
	for i := 0; i < iterations; i++ {
		// Run the boot test once.
		res := perfBootOnce(ctx, s, i, iterations)
		for _, m := range res.Values {
			if m.Multiple {
				pv.Append(perf.Metric{
					Name:      m.Name,
					Unit:      m.Unit,
					Direction: perf.Direction(m.Direction),
					Multiple:  m.Multiple,
				}, m.Value...)
				s.Logf("Logcat event entry: tag=%s unit=%s values=%v", m.Name, m.Unit, m.Value)
			} else {
				metric := perf.Metric{
					Name:      m.Name,
					Unit:      m.Unit,
					Direction: perf.Direction(m.Direction),
					Multiple:  m.Multiple,
				}
				_, ok := singleMetrics[metric]
				if ok {
					singleMetrics[metric] = append(singleMetrics[metric], m.Value...)
				} else {
					singleMetrics[metric] = m.Value
				}
				s.Logf("Logcat event entry: tag=%s unit=%s value=%f", m.Name, m.Unit, m.Value[0])
			}
		}
	}

	for k, values := range singleMetrics {
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		pv.Set(k, sum/float64(len(values)))
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
