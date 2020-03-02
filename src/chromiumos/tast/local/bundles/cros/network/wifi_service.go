// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"reflect"
	"time"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
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

// wifiTestProfileName is the profile we create and use for WiFi tests.
const wifiTestProfileName = "test"

// WifiService implements tast.cros.network.Wifi gRPC service.
type WifiService struct {
	s *testing.ServiceState
}

// InitTestState properly initialize the DUT for WiFi tests.
func (s *WifiService) InitTestState(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	// Stop UI to avoid interference from UI (e.g. request scan).
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		testing.ContextLog(ctx, "Failed to stop ui which might cause troubles, err: ", err)
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	// Turn on WiFi device.
	dev, _, err := m.DevicesByTechnology(ctx, shill.TechnologyWifi)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get WiFi device")
	}
	if len(dev) != 1 {
		return nil, errors.Wrap(err, "multiple WiFi devices found, expect 1")
	}
	dev[0].Enable(ctx)

	// Clean old profiles.
	if err := s.cleanProfiles(ctx); err != nil {
		return nil, errors.Wrap(err, "cleanProfiles failed")
	}
	if err := s.removeWifiEntries(ctx); err != nil {
		return nil, errors.Wrap(err, "removeWifiEntries failed")
	}
	// Try to create test profile.
	m.RemoveProfile(ctx, wifiTestProfileName)
	if _, err := m.CreateProfile(ctx, wifiTestProfileName); err != nil {
		return nil, errors.Wrap(err, "failed to create profile")
	}
	// Push test profile.
	if _, err := m.PushProfile(ctx, wifiTestProfileName); err != nil {
		return nil, errors.Wrap(err, "failed to push profile")
	}
	return &empty.Empty{}, nil
}

// Teardown the settings made by InitTestState.
func (s *WifiService) Teardown(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	var retErr error

	if err := s.cleanProfiles(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "cleanProfiles failed: %s", err.Error())
	}
	if err := s.removeWifiEntries(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "removeWifiEntries failed: %s", err.Error())
	}
	// Try to remove test profile if not yet removed by cleanProfiles.
	if m, err := shill.NewManager(ctx); err != nil {
		retErr = errors.Wrapf(retErr, "failed to create Manager object: %s", err.Error())
	} else {
		m.RemoveProfile(ctx, wifiTestProfileName)
	}
	if err := upstart.StartJob(ctx, "ui"); err != nil {
		retErr = errors.Wrapf(retErr, "failed to start ui: %s", err.Error())
	}
	if retErr != nil {
		return nil, retErr
	}
	return &empty.Empty{}, nil
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
	props := map[string]interface{}{
		shill.ServicePropertyType: shill.TypeWifi,
		shill.ServicePropertyName: config.Ssid,
	}

	// TODO(crbug.com/1034875): collect timing metrics, e.g. discovery time.
	testing.ContextLog(ctx, "Finding service with props: ", props)
	var servicePath dbus.ObjectPath
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		servicePath, err = m.FindMatchingService(ctx, props)
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

	testing.ContextLog(ctx, "Connecting to service with path: ", servicePath)
	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}

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
		Path: string(servicePath),
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

// DeleteEntriesForSSID deletes all WiFi profile entries for a given ssid.
func (s *WifiService) DeleteEntriesForSSID(ctx context.Context, ssid *network.SSID) (*empty.Empty, error) {
	filter := map[string]interface{}{
		shill.ProfileEntryPropertyName: ssid.Ssid,
		shill.ProfileEntryPropertyType: shill.TypeWifi,
	}
	if err := s.removeMatchedEntries(ctx, filter); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// cleanProfiles pops and removes all active profiles until default profile.
func (s *WifiService) cleanProfiles(ctx context.Context) error {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Manager object")
	}
	for {
		profile, err := m.ActiveProfile(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get active profile")
		}
		props, err := profile.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get properties from profile object")
		}
		name, err := props.GetString(shill.ProfilePropertyName)
		if name == shill.DefaultProfileName {
			return nil
		}
		if err != nil {
			return errors.Wrap(err, "failed to get profile name")
		}
		if err := m.PopProfile(ctx, name); err != nil {
			return errors.Wrap(err, "failed to pop profile")
		}
		if err := m.RemoveProfile(ctx, name); err != nil {
			return errors.Wrap(err, "failed to delete profile")
		}
	}
}

// removeWifiEntries removes all the entries with type=wifi in all profiles.
func (s *WifiService) removeWifiEntries(ctx context.Context) error {
	filter := map[string]interface{}{
		shill.ProfileEntryPropertyType: shill.TypeWifi,
	}
	return s.removeMatchedEntries(ctx, filter)
}

// removeMatchedEntries traverses all profiles and removes all entries matching the properties in propFilter.
func (s *WifiService) removeMatchedEntries(ctx context.Context, propFilter map[string]interface{}) error {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Manager object")
	}
	profiles, err := m.Profiles(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get profiles")
	}
	for _, p := range profiles {
		props, err := p.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get properties from profile object")
		}
		entryIDs, err := props.GetStrings(shill.ProfilePropertyEntries)
		if err != nil {
			return errors.Wrapf(err, "failed to get entryIDs from profile %s", p.String())
		}
	entryLoop:
		for _, entryID := range entryIDs {
			entry, err := p.GetEntry(ctx, entryID)
			if err != nil {
				return errors.Wrapf(err, "failed to get entry %s", entryID)
			}
			for k, expect := range propFilter {
				v, ok := entry[k]
				if !ok || !reflect.DeepEqual(expect, v) {
					// not matched, try new entry.
					continue entryLoop
				}
			}
			if err := p.DeleteEntry(ctx, entryID); err != nil {
				return errors.Wrapf(err, "failed to delete entry %s", entryID)
			}
		}
	}
	return nil
}
