// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterWifiServer(srv, &WifiService{s: s})
		},
	})
}

func isConnectedState(state string) bool {
	for _, s := range shill.ServiceConnectedStates {
		if s == state {
			return true
		}
	}
	return false
}

func isIdleState(state string) bool {
	return state == shill.ServiceStateIdle
}

// WifiService implements tast.cros.network.Wifi gRPC service.
type WifiService struct {
	s *testing.ServiceState
}

func (s *WifiService) waitState(ctx context.Context, svc *shill.Service, f func(string) bool) error {
	pw, err := svc.Properties().CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create property watcher")
	}
	defer pw.Close(ctx)
	for {
		props, err := svc.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get service properties")
		}
		v, err := props.Get(shill.ServicePropertyState)
		if err != nil {
			return errors.Wrap(err, "failed to get service state")
		}
		if state, ok := v.(string); !ok {
			return errors.Errorf("unexpected value for ServicePropertyState: %+v", v)
		} else if f(state) {
			return nil
		}
		if err := pw.WaitAll(ctx, shill.ServicePropertyState); err != nil {
			return errors.Wrap(err, "failed to wait service state change")
		}
	}
}

// Connect to a wifi service with specific config.
// This is the implementation of network.Wifi/Connect gRPC.
func (s *WifiService) Connect(ctx context.Context, config *network.Config) (*network.Service, error) {
	testing.ContextLogf(ctx, "Attempting to connect to %s", config.Ssid)

	testing.ContextLog(ctx, "Discovering")
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create manager object")
	}
	props := map[string]interface{}{
		shill.ServicePropertyType: shill.TypeWifi,
		shill.ServicePropertyName: config.Ssid,
	}
	// TODO: May need polling here.
	servicePath, err := m.FindMatchingService(ctx, props)
	if err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Connecting to service with path=%s", servicePath)
	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}
	if err := service.Connect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to service")
	}

	// Wait until connection established.
	if err := s.waitState(ctx, service, isConnectedState); err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "Connected")

	return &network.Service{
		Path: string(servicePath),
	}, nil
}

// Disconnect from a wifi service.
// This is the implementation of network.Wifi/Disconnect gRPC.
func (s *WifiService) Disconnect(ctx context.Context, config *network.Service) (*empty.Empty, error) {
	service, err := shill.NewService(ctx, dbus.ObjectPath(config.Path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}
	if err := service.Disconnect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disconnect")
	}
	testing.ContextLog(ctx, "Wait the service to be idle")
	if err := s.waitState(ctx, service, isIdleState); err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "Disconected")
	return &empty.Empty{}, nil
}
