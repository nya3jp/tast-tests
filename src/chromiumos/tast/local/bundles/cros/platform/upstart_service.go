// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			platform.RegisterUpstartServiceServer(srv, &UpstartService{s})
		},
	})
}

// UpstartService implements tast.cros.platform.UpstartService.
type UpstartService struct {
	s *testing.ServiceState
}

// CheckJob validates that the given upstart job is running.
func (*UpstartService) CheckJob(ctx context.Context, request *platform.CheckJobRequest) (*empty.Empty, error) {
	return &empty.Empty{}, upstart.CheckJob(ctx, request.JobName)
}

// JobStatus returns the current status of job.
// If the PID is unavailable (i.e. the process is not running), 0 will be returned.
// An error will be returned if the job is unknown (i.e. it has no config in /etc/init).
func (*UpstartService) JobStatus(ctx context.Context, request *platform.JobStatusRequest) (*platform.JobStatusResponse, error) {
	goal, state, pid, err := upstart.JobStatus(ctx, request.JobName)
	return &platform.JobStatusResponse{
		Goal:  string(goal),
		State: string(state),
		Pid:   int32(pid),
	}, err
}

// StartJob starts job. If it is already running, this returns an error.
func (*UpstartService) StartJob(ctx context.Context, request *platform.StartJobRequest) (*empty.Empty, error) {
	var args []upstart.Arg
	for _, arg := range request.GetArgs() {
		args = append(args, upstart.WithArg(arg.GetKey(), arg.GetValue()))
	}
	return &empty.Empty{}, upstart.StartJob(ctx, request.JobName, args...)
}

// StopJob stops job. If it is not currently running, this is a no-op.
func (*UpstartService) StopJob(ctx context.Context, request *platform.StopJobRequest) (*empty.Empty, error) {
	return &empty.Empty{}, upstart.StopJob(ctx, request.JobName)
}

// EnableJob enables an upstart job that was previously disabled.
func (*UpstartService) EnableJob(ctx context.Context, request *platform.EnableJobRequest) (*empty.Empty, error) {
	return &empty.Empty{}, upstart.EnableJob(request.JobName)
}

// DisableJob disables an upstart job, which takes effect on the next reboot.
func (*UpstartService) DisableJob(ctx context.Context, request *platform.DisableJobRequest) (*empty.Empty, error) {
	return &empty.Empty{}, upstart.DisableJob(request.JobName)
}

// IsJobEnabled checks if the given upstart job is enabled.
func (*UpstartService) IsJobEnabled(ctx context.Context, request *platform.IsJobEnabledRequest) (*platform.IsJobEnabledResponse, error) {
	enabled, err := upstart.IsJobEnabled(request.JobName)
	return &platform.IsJobEnabledResponse{Enabled: enabled}, err
}
