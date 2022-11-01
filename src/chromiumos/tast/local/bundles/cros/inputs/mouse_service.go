// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/common"
	"chromiumos/tast/local/input"
	pb "chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/testing"
)

func init() {
	var mouseService MouseService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			mouseService = MouseService{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterMouseServiceServer(srv, &mouseService)
		},
		GuaranteeCompatibility: true,
	})
}

// MouseService implements tast.cros.inputs.MouseService.
type MouseService struct {
	sharedObject *common.SharedObjectsForService
	mutex        sync.Mutex
	mouse        *input.MouseEventWriter
}

func (svc *MouseService) NewMouse(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	mdvc, err := input.Mouse(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mouse handle")
	}

	svc.mouse = mdvc
	return &empty.Empty{}, nil
}

func (svc *MouseService) CloseMouse(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if svc.mouse == nil {
		return nil, errors.New("CloseMouse called before New")
	}

	svc.mouse.Close()
	return &empty.Empty{}, nil
}
