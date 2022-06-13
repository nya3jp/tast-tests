// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/perf/perfpb"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/perfboot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/memory/metrics"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterPerfBootServiceServer(srv, &PerfBootService{s: s})
		},
	})
}

// PerfBootService implements tast.cros.arc.PerfBootService.
type PerfBootService struct {
	s *testing.ServiceState
}

func (c *PerfBootService) WaitUntilCPUCoolDown(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownStopUI)); err != nil {
		return nil, errors.Wrap(err, "failed to wait until CPU is cooled down")
	}
	return &empty.Empty{}, nil
}

func (c *PerfBootService) GetPerfValues(ctx context.Context, req *empty.Empty) (*perfpb.Values, error) {
	// TODO(niwa): Check if we should use GAIA login instead of fake login.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.RestrictARCCPU(),
		chrome.ExtraArgs("--disable-arc-data-wipe", "--ignore-arcvm-dev-conf"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	defer os.RemoveAll(td)

	a, err := arc.New(ctx, td)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating test API connection failed")
	}

	p, err := perfboot.GetPerfValues(ctx, tconn, a)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract perf metrics")
	}

	result := perf.NewValues()
	for k, v := range p {
		result.Set(perf.Metric{
			Name:      k,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, float64(v.Milliseconds()))
	}

	if err := metrics.LogMemoryStats(ctx, nil, a, result, "", ""); err != nil {
		return nil, errors.Wrap(err, "failed to collect memory metrics")
	}

	return result.Proto(), nil
}
