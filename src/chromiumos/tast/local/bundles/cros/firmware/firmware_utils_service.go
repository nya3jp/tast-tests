// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/firmware"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			fwpb.RegisterUtilsServiceServer(srv, &UtilsService{s: s})
		},
	})
}

// UtilsService implements tast.cros.firmware.UtilsService.
type UtilsService struct {
	s *testing.ServiceState
}

// CheckBootMode wraps a call to the local firmware support package.
func (*UtilsService) CheckBootMode(ctx context.Context, req *fwpb.CheckBootModeRequest) (*fwpb.CheckBootModeResponse, error) {
	var mode int
	switch req.BootMode {
	case fwpb.BootMode_BOOT_MODE_UNSPECIFIED:
		return nil, errors.New("cannot check unspecified boot mode")
	case fwpb.BootMode_BOOT_MODE_NORMAL:
		mode = firmware.BootModeNormal
	case fwpb.BootMode_BOOT_MODE_DEV:
		mode = firmware.BootModeDev
	case fwpb.BootMode_BOOT_MODE_RECOVERY:
		mode = firmware.BootModeRecovery
	default:
		return nil, errors.Errorf("did not recognize boot mode %v", req.BootMode)
	}
	verified, err := firmware.CheckBootMode(ctx, mode)
	if err != nil {
		return nil, err
	}
	return &fwpb.CheckBootModeResponse{Verified: verified}, nil
}
