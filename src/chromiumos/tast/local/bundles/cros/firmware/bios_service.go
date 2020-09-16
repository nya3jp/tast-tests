// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/firmware/bios"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterBiosServiceServer(srv, &BiosService{s: s})
		},
	})
}

// BiosService implements tast.cros.firmware.BiosService.
type BiosService struct {
	s *testing.ServiceState
}

// GetGBBFlags gets the flags that are cleared and set.
func (*BiosService) GetGBBFlags(ctx context.Context, req *empty.Empty) (*pb.GBBFlagsState, error) {
	img, err := bios.NewImage(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware")
	}
	cf, sf, err := img.GetGBBFlags()
	if err != nil {
		return nil, errors.Wrap(err, "could not get GBB flags")
	}
	ret := pb.GBBFlagsState{Clear: cf, Set: sf}
	return &ret, nil
}
