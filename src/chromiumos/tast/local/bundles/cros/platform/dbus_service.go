// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			platform.RegisterDbusServiceServer(srv, &DbusService{s})
		},
	})
}

// DbusService implements tast.cros.platform.DbusService.
type DbusService struct {
	s *testing.ServiceState
}

// EnableDbusActivation enables DBus activation for given service.
func (*DbusService) EnableDbusActivation(ctx context.Context, request *platform.EnableDbusActivationRequest) (*empty.Empty, error) {
	return &empty.Empty{}, dbusutil.EnableDbusActivation(ctx, request.ServiceName)
}

// DisableDbusActivation disables DBus activation for given service.
func (*DbusService) DisableDbusActivation(ctx context.Context, request *platform.DisableDbusActivationRequest) (*empty.Empty, error) {
	return &empty.Empty{}, dbusutil.DisableDbusActivation(ctx, request.ServiceName)
}

// IsDbusActivationEnabled checks if given service has bus activation enabled.
func (*DbusService) IsDbusActivationEnabled(ctx context.Context, request *platform.IsDbusActivationEnabledRequest) (*platform.IsDbusActivationEnabledResponse, error) {
	enabled, err := dbusutil.IsDbusActivationEnabled(request.ServiceName)
	return &platform.IsDbusActivationEnabledResponse{Enabled: enabled}, err
}
