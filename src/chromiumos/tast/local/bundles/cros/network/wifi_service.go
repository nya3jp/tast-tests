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

	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterWifiServiceServer(srv, &WifiService{s: s})
		},
	})
}

// wifiTestProfileName is the profile we create and use for WiFi tests.
const wifiTestProfileName = "test"

// WifiService implements tast.cros.network.Wifi gRPC service.
type WifiService struct {
	s *testing.ServiceState
}

// InitDUT properly initializes the DUT for WiFi tests.
func (s *WifiService) InitDUT(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	// Stop UI to avoid interference from UI (e.g. request scan).
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return nil, errors.Wrap(err, "failed to stop ui")
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	// Turn on the WiFi device.
	iface, err := shill.WifiInterface(ctx, m, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get a WiFi device")
	}
	dev, err := m.DeviceByName(ctx, iface)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find the device for interface %s", iface)
	}
	if err := dev.Enable(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to enable WiFi device")
	}
	if err := s.reinitTestState(ctx, m); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// reinitTestState prepare the environment for WiFi testcase.
func (s *WifiService) reinitTestState(ctx context.Context, m *shill.Manager) error {
	// Clean old profiles.
	if err := s.cleanProfiles(ctx, m); err != nil {
		return errors.Wrap(err, "cleanProfiles failed")
	}
	if err := s.removeWifiEntries(ctx, m); err != nil {
		return errors.Wrap(err, "removeWifiEntries failed")
	}
	// Try to create the test profile.
	if _, err := m.CreateProfile(ctx, wifiTestProfileName); err != nil {
		return errors.Wrap(err, "failed to create the test profile")
	}
	// Push the test profile.
	if _, err := m.PushProfile(ctx, wifiTestProfileName); err != nil {
		return errors.Wrap(err, "failed to push the test profile")
	}
	return nil
}

// ReinitTestState cleans and sets up the environment for a single WiFi testcase.
func (s *WifiService) ReinitTestState(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	if err := s.reinitTestState(ctx, m); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// TearDown reverts the settings made by InitDUT and InitTestState.
func (s *WifiService) TearDown(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}

	var retErr error
	if err := s.cleanProfiles(ctx, m); err != nil {
		retErr = errors.Wrapf(retErr, "cleanProfiles failed: %s", err)
	}
	if err := s.removeWifiEntries(ctx, m); err != nil {
		retErr = errors.Wrapf(retErr, "removeWifiEntries failed: %s", err)
	}
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		testing.ContextLog(ctx, "Failed to start ui: ", err)
	}
	if retErr != nil {
		return nil, retErr
	}
	return &empty.Empty{}, nil
}

// Connect connects to a WiFi service with specific config.
// This is the implementation of network.Wifi/Connect gRPC.
func (s *WifiService) Connect(ctx context.Context, request *network.ConnectRequest) (*network.ConnectResponse, error) {
	testing.ContextLog(ctx, "Attempting to connect with config: ", request)

	testing.ContextLog(ctx, "Discovering")
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a manager object")
	}

	// Configure a service for the hidden SSID as a result of manual input SSID.
	if request.Hidden {
		props := map[string]interface{}{
			shill.ServicePropertyType:           shill.TypeWifi,
			shill.ServicePropertySSID:           request.Ssid,
			shill.ServicePropertyWiFiHiddenSSID: request.Hidden,
			shill.ServicePropertySecurityClass:  request.Security,
		}
		if err := m.ConfigureService(ctx, props); err != nil {
			return nil, errors.Wrap(err, "failed to configure a hidden SSID")
		}
	}

	props := map[string]interface{}{
		shill.ServicePropertyType:          shill.TypeWifi,
		shill.ServicePropertyName:          request.Ssid,
		shill.ServicePropertySecurityClass: request.Security,
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

	shillProps, err := protoutil.DecodeFromShillValMap(request.Shillprops)
	if err != nil {
		return nil, err
	}
	for k, v := range shillProps {
		if err = service.SetProperty(ctx, k, v); err != nil {
			return nil, errors.Wrapf(err, "failed to set properties %s to %v", k, v)
		}
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

	return &network.ConnectResponse{
		ServicePath: string(service.ObjectPath()),
	}, nil
}

// Disconnect disconnects from a WiFi service.
// This is the implementation of network.Wifi/Disconnect gRPC.
func (s *WifiService) Disconnect(ctx context.Context, request *network.DisconnectRequest) (*empty.Empty, error) {
	service, err := shill.NewService(ctx, dbus.ObjectPath(request.ServicePath))
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

// QueryService queries shill service information.
// This is the implementation of network.Wifi/QueryService gRPC.
func (s *WifiService) QueryService(ctx context.Context, ser *network.QueryServiceRequest) (*network.QueryServiceResponse, error) {
	service, err := shill.NewService(ctx, dbus.ObjectPath(ser.Path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}
	props, err := service.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service properties")
	}

	hidden, err := props.GetBool(shill.ServicePropertyWiFiHiddenSSID)
	if err != nil {
		return nil, err
	}

	return &network.QueryServiceResponse{
		Hidden: hidden,
	}, nil
}

// DeleteEntriesForSSID deletes all WiFi profile entries for a given ssid.
func (s *WifiService) DeleteEntriesForSSID(ctx context.Context, request *network.DeleteEntriesForSSIDRequest) (*empty.Empty, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	filter := map[string]interface{}{
		shill.ProfileEntryPropertyName: request.Ssid,
		shill.ProfileEntryPropertyType: shill.TypeWifi,
	}
	if err := s.removeMatchedEntries(ctx, m, filter); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// cleanProfiles pops and removes all active profiles until default profile and
// then removes the WiFi test profile if still exists.
func (s *WifiService) cleanProfiles(ctx context.Context, m *shill.Manager) error {
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
			break
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
	// Try to remove the test profile.
	m.RemoveProfile(ctx, wifiTestProfileName)
	return nil
}

// removeWifiEntries removes all the entries with type=wifi in all profiles.
func (s *WifiService) removeWifiEntries(ctx context.Context, m *shill.Manager) error {
	filter := map[string]interface{}{
		shill.ProfileEntryPropertyType: shill.TypeWifi,
	}
	return s.removeMatchedEntries(ctx, m, filter)
}

// removeMatchedEntries traverses all profiles and removes all entries matching the properties in propFilter.
func (s *WifiService) removeMatchedEntries(ctx context.Context, m *shill.Manager, propFilter map[string]interface{}) error {
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
