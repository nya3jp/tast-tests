// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

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

// WifiService implements tast.cros.network.Wifi gRPC service.
type WifiService struct {
	s *testing.ServiceState
}

// Connect connects to a WiFi service with specific config.
// This is the implementation of network.Wifi/Connect gRPC.
func (s *WifiService) Connect(ctx context.Context, config *network.Config) (*network.Service, error) {
	testing.ContextLog(ctx, "Attempting to connect with config: ", config)

	testing.ContextLog(ctx, "Discovering")
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a manager object")
	}

	// Configure a service for the hidden SSID as a result of manual input SSID.
	if config.Hidden {
		props := map[string]interface{}{
			shill.ServicePropertyType:           shill.TypeWifi,
			shill.ServicePropertySSID:           config.Ssid,
			shill.ServicePropertyWiFiHiddenSSID: config.Hidden,
		}
		if err := m.ConfigureService(ctx, props); err != nil {
			return nil, errors.Wrap(err, "failed to configure a hidden SSID")
		}
	}

	props := map[string]interface{}{
		shill.ServicePropertyType: shill.TypeWifi,
		shill.ServicePropertyName: config.Ssid,
	}

	// TODO(crbug.com/1034875): collect timing metrics, e.g. discovery time.
	testing.ContextLog(ctx, "Finding service with props: ", props)
	var service *shill.Service
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		service, err = m.FindMatchingService(ctx, props)
		if err == nil {
			return nil
		}
		// Scan WiFi AP again if the expected AP is not found.
		if err2 := m.RequestScan(ctx, shill.TechnologyWifi); err2 != nil {
			return testing.PollBreak(errors.Wrap(err2, "failed to request active scan"))
		}
		return err
	}, &testing.PollOptions{
		Timeout:  15 * time.Second,
		Interval: 200 * time.Millisecond, // RequestScan is spammy, but shill handles that for us.
	}); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Connecting to service: ", service)

	// Spawn watcher before connect.
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	if err := service.Connect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to service")
	}

	// Wait until connection established.
	// According to previous Autotest tests, a reasonable timeout is
	// 15 seconds for association and 15 seconds for configuration.
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := pw.Expect(timeoutCtx, shill.ServicePropertyIsConnected, true); err != nil {
		return nil, err
	}

	return &network.Service{
		Path: string(service.ObjectPath()),
	}, nil
}

// Disconnect disconnects from a WiFi service.
// This is the implementation of network.Wifi/Disconnect gRPC.
func (s *WifiService) Disconnect(ctx context.Context, config *network.Service) (*empty.Empty, error) {
	service, err := shill.NewService(ctx, dbus.ObjectPath(config.Path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}
	// Spawn watcher before disconnect.
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)
	if err := service.Disconnect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disconnect")
	}
	testing.ContextLog(ctx, "Wait for the service to be idle")
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pw.Expect(timeoutCtx, shill.ServicePropertyState, shill.ServiceStateIdle); err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "Disconnected")
	return &empty.Empty{}, nil
}

// RemoveServiceBySSID removes WiFi service for a given SSID.
func (s *WifiService) RemoveServiceBySSID(ctx context.Context, ssid *network.SSID) (*empty.Empty, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a manager object")
	}
	service, err := m.FindAnyMatchingService(ctx, map[string]interface{}{
		shill.ServicePropertyType: shill.TypeWifi,
		shill.ServicePropertyName: ssid.Ssid,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find matching service")
	}
	testing.ContextLogf(ctx, "Removing service %s", service)
	if err = service.Remove(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to remove service %s", service)
	}
	return &empty.Empty{}, nil
}
