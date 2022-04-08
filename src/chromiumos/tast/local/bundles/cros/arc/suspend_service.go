// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes/duration"
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

// SuspendService implements tast.cros.arc.SuspendService
type SuspendService struct {
	s *testing.ServiceState
}

type readclocksOutput struct {
	Boot duration.Duration `json:"CLOCK_BOOTTIME"`
	Mono duration.Duration `json:"CLOCK_MONOTONIC"`
}

const hostReadClocksPath = "/usr/local/libexec/tast/helpers/local/cros/arc.Suspend.readclocks"

// Prepare restarts the ui service on DUT and deploys a binary into ARC to monitor the guest clocks.
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
	res.ClockMonotonic = &clocks.Mono
	res.ClockBoottime = &clocks.Boot
	return res, nil
}

func readARCClocks(ctx context.Context, readClocksPath string) (*arcpb.ClockValues, error) {
	// This will take some time since it creates a connection to ARC again
	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	defer os.RemoveAll(td)

	// Reestablish a connection to ARC since the service state will be lost
	// when the RPC connection is renewed.
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

func readHostDate(ctx context.Context) (*string, error) {
	output, err := testexec.CommandContext(
		ctx,
		"date",
	).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run date command on host")
	}
	ret := string(output)
	return &ret, nil
}

func readARCDate(ctx context.Context) (*string, error) {
	// This will take some time since it creates a connection to ARC again
	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	defer os.RemoveAll(td)

	// Reestablish a connection to ARC since the service state will be lost
	// when the RPC connection is renewed.
	a, err := arc.New(ctx, td)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	defer a.Close(ctx)

	output, err := a.Command(
		ctx,
		"date",
	).Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run date command on ARC")
	}
	ret := string(output)
	return &ret, nil
}

// GetClockValues returns the current values of CLOCK_MONOTONINC and CLOCK_BOOTTIME in the guest and the host.
func (c *SuspendService) GetDateOutputs(ctx context.Context, params *arcpb.SuspendServiceParams) (*arcpb.GetDateOutputsResponse, error) {
	res := &arcpb.GetDateOutputsResponse{}

	date, err := readHostDate(ctx)
	if err != nil {
		return nil, err
	}
	res.Host = *date

	date, err = readARCDate(ctx)
	if err != nil {
		return nil, err
	}
	res.Arc = *date

	return res, nil
}

// GetClockValues returns the current values of CLOCK_MONOTONINC and CLOCK_BOOTTIME in the guest and the host.
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
