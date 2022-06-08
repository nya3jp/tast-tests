// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterChromeServiceServer(srv,
				&ChromeService{sharedObject: common.SharedObjectsForServiceSingleton})
		},
		GuaranteeCompatibility: true,
	})
}

// ChromeService implements tast.cros.ui.ChromeService
type ChromeService struct {
	sharedObject *common.SharedObjectsForService
}

// New logs into Chrome with the supplied chrome options.
func (svc *ChromeService) New(ctx context.Context, req *pb.NewRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	opts, err := chrome.ToOptions(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to chrome options")
	}

	// By default, this will always create a new chrome session even when there is an existing one.
	// This gives full control of the lifecycle to the end users.
	// Users can use TryReuseSessions if they want to potentially reuse the session.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		testing.ContextLog(ctx, "Failed to start Chrome")
		return nil, err
	}

	// Store the newly created chrome sessions in the shared object so other services can use it.
	svc.sharedObject.Chrome = cr

	return &empty.Empty{}, nil
}

// Close closes all surfaces and Chrome.
// This will likely be called in a defer in remote tests instead of called explicitly.
func (svc *ChromeService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	if svc.sharedObject.Chrome == nil {
		return nil, errors.New("Chrome not available")
	}

	err := svc.sharedObject.Chrome.Close(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome: ", err)
	}

	svc.sharedObject.Chrome = nil
	return &empty.Empty{}, err
}
