// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/local/firmware"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			fwpb.RegisterFpUpdaterServiceServer(srv, &FpUpdaterService{s: s})
		},
	})
}

// FpUpdaterService implements tast.cros.firmware.FpUpdaterService.
type FpUpdaterService struct {
	s *testing.ServiceState
}

// ReadUpdaterLogs reads the latest and previous logs from the fingerprint firmware updater.
func (*FpUpdaterService) ReadUpdaterLogs(ctx context.Context, req *empty.Empty) (*fwpb.ReadFpUpdaterLogsResponse, error) {
	latestData, previousData, err := firmware.ReadFpUpdaterLogs()
	if err != nil {
		return nil, err
	}
	return &fwpb.ReadFpUpdaterLogsResponse{LatestLog: latestData, PreviousLog: previousData}, nil
}
