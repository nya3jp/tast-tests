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

// ClearSetGBBFlags clears and sets specified GBB flags, leaving the rest unchanged.
func (s *BiosService) ClearSetGBBFlags(ctx context.Context, req *pb.GBBFlagsState) (*empty.Empty, error) {
	//time.Sleep(12 * time.Second)
	//return nil, errors.Wrap(nil, "skipping service")
	img, err := bios.NewImage(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not read firmware")
	}
	s.s.Log("Start ClearSetGBBFlags, clear: ", req.Clear, " set: ", req.Set)
	err = img.ClearSetGBBFlags(req.Clear, req.Set)
	if err != nil {
		return nil, errors.Wrap(err, "could not clear/set flags")
	}
	err = img.WriteSection(ctx, bios.GBBImageSection)
	if err != nil {
		return nil, errors.Wrap(err, "could not write image")
	}
	return &empty.Empty{}, nil
}
