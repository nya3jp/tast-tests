// Copyright 2019 The Chromium OS Authors. All rights reserved.
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

func (s *WifiService) waitState(ctx context.Context, svc *shill.Service,
	pw *shill.PropertiesWatcher, f func(string) bool) error {
	for {
		v, err := svc.Properties().Get(shill.ServicePropertyState)
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

	testing.ContextLogf(ctx, "Finding service with props=%v", props)
	// TODO: hmm..., I wonder if we'll prefer the old FindMatchingService here, with which
	// we don't need to spawn goroutine and can have simple polling.
	var servicePath dbus.ObjectPath
	waitDone := make(chan error)
	go func() {
		// TODO: properly set timeout for this.
		servicePath, err = m.WaitForServiceProperties(ctx, props, 10*time.Second)
		close(waitDone)
	}()

	ticker := time.NewTicker(3 * time.Second) // Active scan can take up to 1.5s.
WaitLoop:
	for {
		select {
		case <-ticker.C:
			err := m.RequestScan(ctx, shill.TechnologyWifi)
			if err != nil {
				testing.ContextLog(ctx, "Failed to request active scan: ", err)
			}
		case <-waitDone:
			break WaitLoop
		}
	}
	ticker.Stop()

	if err != nil {
		return nil, errors.Wrap(err, "failed to find matched service")
	}

	testing.ContextLogf(ctx, "Connecting to service with path=%s", servicePath)
	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}

	// Spawn watcher before connect.
	pw, err := service.Properties().CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	if err := service.Connect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to service")
	}

	// Wait until connection established.
	if err := s.waitState(ctx, service, pw, isConnectedState); err != nil {
		return nil, err
	}

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
	// Spawn watcher before disconnect.
	pw, err := service.Properties().CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)
	if err := service.Disconnect(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disconnect")
	}
	testing.ContextLog(ctx, "Wait the service to be idle")
	if err := s.waitState(ctx, service, pw, isIdleState); err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "Disconected")
	return &empty.Empty{}, nil
}

// DeleteEntriesForSSID deletes all profile entries for a given ssid.
func (s *WifiService) DeleteEntriesForSSID(ctx context.Context, ssid *network.SSID) (*empty.Empty, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create manager object")
	}
	paths, err := m.GetProfiles(ctx)
	for _, path := range paths {
		profile, err := shill.NewProfile(ctx, path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create profile object on path=%s", path)
		}
		v, err := profile.Properties().Get(shill.ProfilePropertyEntries)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get property %s from profile object", shill.ProfilePropertyEntries)
		}
		entryIDs, ok := v.([]string)
		if !ok {
			return nil, errors.Errorf("unexpected value %v", v)
		}
		for _, entryID := range entryIDs {
			entry, err := profile.GetEntry(ctx, entryID)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get entry %s", entryID)
			}
			if entry[shill.ProfileEntryPropertyName] != ssid.Ssid {
				continue
			}
			err = profile.DeleteEntry(ctx, entryID)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to delete entry %s", entryID)
			}
		}
	}
	return &empty.Empty{}, nil
}
