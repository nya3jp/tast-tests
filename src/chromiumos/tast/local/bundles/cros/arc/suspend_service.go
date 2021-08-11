// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterSuspendServiceServer(srv, &SuspendService{s: s})
		},
	})
}

type SuspendService struct {
	s *testing.ServiceState
}

type readclocksTimespec struct {
	Seconds     int64 `json:"tv_sec"`
	NanoSeconds int64 `json:"tv_nsec"`
}

type readclocksOutput struct {
	Boot readclocksTimespec `json:"CLOCK_BOOTTIME"`
	Mono readclocksTimespec `json:"CLOCK_MONOTONIC"`
	TSC  int64
}

const hostReadClocksPath = "/usr/local/libexec/tast/helpers/local/cros/arc.Suspend.readclocks"

func (c *SuspendService) Prepare(ctx context.Context, req *empty.Empty) (*arcpb.SuspendServiceParams, error) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.RestrictARCCPU(),
		chrome.KeepState(), chrome.ExtraArgs("--disable-arc-data-wipe", "--ignore-arcvm-dev-conf"))
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

	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check whether ARCVM is enabled")
	}
	if vmEnabled == false {
		return nil, errors.Wrap(err, "this test is only for VMs")
	}

	readclocksPath, err := a.PushFileToTmpDir(ctx, hostReadClocksPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to push test binary to ARC")
	}

	if err := a.Command(ctx, "chmod", "0777", readclocksPath).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to change test binary permissions")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating test API connection failed")
	}
	defer tconn.Close()

	act, err := arc.NewActivity(a, "com.google.android.deskclock", "com.android.deskclock.DeskClock")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new activity")
	}
	if err := act.Start(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed start Settings activity")
	}

	res := &arcpb.SuspendServiceParams{}
	res.ReadClocksPathInArc = readclocksPath
	return res, nil
}

func parseReadClocksOutput(output []byte) (*arcpb.ClockValues, error) {
	var clocks readclocksOutput
	err := json.Unmarshal(output, &clocks)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse readclocks output")
	}

	res := &arcpb.ClockValues{}
	res.ClockMonotonicNs = clocks.Mono.Seconds*secToNs + clocks.Mono.NanoSeconds
	res.ClockBoottimeNs = clocks.Boot.Seconds*secToNs + clocks.Boot.NanoSeconds
	res.Tsc = clocks.TSC
	return res, nil
}

const secToNs = 1000 * 1000 * 1000

func readARCClocks(ctx context.Context, readClocksPath string) (*arcpb.ClockValues, error) {
	// This will take some time since it creates a connection to ARC again
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

	output, err := a.Command(ctx, readClocksPath).Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run readclocks binary on ARC")
	}
	return parseReadClocksOutput(output)
}

func readHostClocks(ctx context.Context) (*arcpb.ClockValues, error) {
	output, err := testexec.CommandContext(ctx, hostReadClocksPath).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run readclocks binary on host")
	}
	return parseReadClocksOutput(output)
}

func (c *SuspendService) GetClockValues(ctx context.Context, params *arcpb.SuspendServiceParams) (*arcpb.GetClockValuesResponse, error) {
	res := &arcpb.GetClockValuesResponse{}

	clock, err := readHostClocks(ctx)
	if err != nil {
		return nil, err
	}
	res.Host = clock

	clock, err = readARCClocks(ctx, params.ReadClocksPathInArc)
	if err != nil {
		return nil, err
	}
	res.Arc = clock

	return res, nil
}
