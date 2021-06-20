// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/network/protoutil"
	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/network/cmd"
	network_iface "chromiumos/tast/local/network/iface"
	local_ping "chromiumos/tast/local/network/ping"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/wpasupplicant"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// reserveForReturn reserves a second in order to let the gRPC to be able to return the details of the errors (mainly for timeout errors).
func reserveForReturn(ctx context.Context) (context.Context, func()) {
	return ctxutil.Shorten(ctx, time.Second)
}

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			wifi.RegisterShillServiceServer(srv, &ShillService{s: s})
		},
	})
}

// wifiTestProfileName is the profile we create and use for WiFi tests.
const wifiTestProfileName = "test"

// ShillService implements tast.cros.wifi.Shill gRPC service.
type ShillService struct {
	s *testing.ServiceState
}

// InitDUT properly initializes the DUT for WiFi tests.
func (s *ShillService) InitDUT(ctx context.Context, req *wifi.InitDUTRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	if !req.WithUi {
		// Stop UI to avoid interference from UI (e.g. request scan).
		if err := upstart.StopJob(ctx, "ui"); err != nil {
			return nil, errors.Wrap(err, "failed to stop ui")
		}
	} else {
		if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
			return nil, errors.Wrap(err, "failed to start ui")
		}
	}

	m, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
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
func (s *ShillService) reinitTestState(ctx context.Context, m *shill.Manager) error {
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
	// Clean up wpa_supplicant BSSID_IGNORE in case some BSSID cannot be scanned.
	// See https://crrev.com/c/219844.
	if err := wpacli.NewRunner(&cmd.LocalCmdRunner{}).ClearBSSIDIgnore(ctx); err != nil {
		return errors.Wrap(err, "failed to clear wpa_supplicant BSSID_IGNORE")
	}
	return nil
}

// ReinitTestState cleans and sets up the environment for a single WiFi testcase.
func (s *ShillService) ReinitTestState(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	if err := s.reinitTestState(ctx, m); err != nil {
		return nil, err
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		servicePath, err := s.selectedService(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if servicePath != "/" {
			return errors.Errorf("unexpected service path, got %q, want \"/\"", servicePath)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  time.Second * 5,
		Interval: time.Millisecond * 200,
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// TearDown reverts the settings made by InitDUT and InitTestState.
func (s *ShillService) TearDown(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
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

func (s *ShillService) discoverService(ctx context.Context, m *shill.Manager, props map[string]interface{}) (*shill.Service, error) {
	ctx, st := timing.Start(ctx, "discoverService")
	defer st.End()
	testing.ContextLog(ctx, "Discovering a WiFi service with properties: ", props)

	visibleProps := make(map[string]interface{})
	for k, v := range props {
		visibleProps[k] = v
	}
	visibleProps[shillconst.ServicePropertyVisible] = true

	var service *shill.Service
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		service, err = m.FindMatchingService(ctx, visibleProps)
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
	return service, nil
}

// connectService connects to a WiFi service and wait until conntected state.
// The time used for association and configuration is returned when success.
func (s *ShillService) connectService(ctx context.Context, service *shill.Service) (assocTime, configTime time.Duration, retErr error) {
	ctx, st := timing.Start(ctx, "connectService")
	defer st.End()
	testing.ContextLog(ctx, "Connecting to the service: ", service)

	start := time.Now()

	// Spawn watcher before connect.
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	if err := service.Connect(ctx); err != nil {
		return 0, 0, errors.Wrap(err, "failed to connect to service")
	}

	// Wait until connection established.
	// For debug and profile purpose, it is separated into association
	// and configuration stages.

	// Prepare the state list for ExpectIn.
	var connectedStates []interface{}
	for _, s := range shillconst.ServiceConnectedStates {
		connectedStates = append(connectedStates, s)
	}
	associatedStates := append(connectedStates, shillconst.ServiceStateConfiguration)

	testing.ContextLog(ctx, "Associating with ", service)
	assocCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	state, err := pw.ExpectIn(assocCtx, shillconst.ServicePropertyState, associatedStates)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to associate")
	}
	assocTime = time.Since(start)
	start = time.Now()

	testing.ContextLog(ctx, "Configuring ", service)
	if state == shillconst.ServiceStateConfiguration {
		// We're not yet in connectedStates, wait until connected.
		configCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if _, err := pw.ExpectIn(configCtx, shillconst.ServicePropertyState, connectedStates); err != nil {
			return 0, 0, errors.Wrap(err, "failed to configure")
		}
	}
	configTime = time.Since(start)

	return assocTime, configTime, nil
}

// waitForBSSID waits for a BSS with specific SSID and BSSID on the
// given iface. Returns error if it fails to wait for the BSS before
// ctx.Done.
func (s *ShillService) waitForBSSID(ctx context.Context, iface *wpasupplicant.Interface, targetSSID, targetBSSID []byte) error {
	// Create a watcher for BSSAdded signal.
	sw, err := iface.DBusObject().CreateWatcher(ctx, wpasupplicant.DBusInterfaceSignalBSSAdded)
	if err != nil {
		return errors.Wrap(err, "failed to create a signal watcher")
	}
	defer sw.Close(ctx)

	// Check if the BSS is already in the table.
	bsses, err := iface.BSSs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get BSSs")
	}

	for _, bss := range bsses {
		ssid, err := bss.SSID(ctx)
		if err != nil {
			testing.ContextLog(ctx, "Failed to get SSID for bss: ", err)
		} else if bytes.Equal(ssid, targetSSID) {
			bssid, err := bss.BSSID(ctx)
			if err != nil {
				testing.ContextLog(ctx, "Failed to get BSSID for bss: ", err)
			} else if bytes.Equal(bssid, targetBSSID) {
				return nil
			}
		}
	}

	checkSig := func(sig *dbus.Signal) (bool, error) {
		bss, err := iface.ParseBSSAddedSignal(ctx, sig)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse the BSSAdded signal")
		}
		if !bytes.Equal(bss.SSID, targetSSID) {
			return false, nil
		}
		if !bytes.Equal(bss.BSSID, targetBSSID) {
			return false, nil
		}
		return true, nil
	}

	for {
		select {
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "failed to wait for BSSID %q", targetBSSID)
		case sig := <-sw.Signals:
			match, err := checkSig(sig)
			if err != nil {
				return err
			} else if match {
				return nil
			}
		}
	}
}

// DiscoverBSSID discovers the specified BSSID by running a scan.
// This is the implementation of wifi.ShillService/DiscoverBSSID gRPC.
func (s *ShillService) DiscoverBSSID(ctx context.Context, request *wifi.DiscoverBSSIDRequest) (*wifi.DiscoverBSSIDResponse, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	supplicant, err := wpasupplicant.NewSupplicant(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to wpa_supplicant")
	}
	iface, err := supplicant.GetInterface(ctx, request.Interface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interface object paths")
	}

	// Convert BSSID from human readable string to hardware address in bytes.
	requestBSSID, err := net.ParseMAC(request.Bssid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse the MAC address from %s", request.Bssid)
	}

	start := time.Now()

	// Start background routine to wait for the expected BSS.
	done := make(chan error, 1)
	// Wait for bg routine to end.
	defer func() { <-done }()
	// Notify the bg routine to end if we return early.
	bgCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func(ctx context.Context) {
		defer close(done)
		done <- s.waitForBSSID(ctx, iface, request.Ssid, requestBSSID)
	}(bgCtx)

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager")
	}
	// Trigger request scan every 200ms if the expected BSS is not found.
	// It might be spammy, but shill handles it for us.
	for {
		if err := m.RequestScan(ctx, shill.TechnologyWifi); err != nil {
			return nil, err
		}
		select {
		case err := <-done:
			if err != nil {
				return nil, err
			}
			discoveryTime := time.Since(start)
			return &wifi.DiscoverBSSIDResponse{
				DiscoveryTime: discoveryTime.Nanoseconds(),
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Connect connects to a WiFi service with specific config.
// This is the implementation of wifi.ShillService/Connect gRPC.
func (s *ShillService) Connect(ctx context.Context, request *wifi.ConnectRequest) (*wifi.ConnectResponse, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	ctx, st := timing.Start(ctx, "wifi_service.Connect")
	defer st.End()
	testing.ContextLog(ctx, "Attempting to connect with config: ", request)

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a manager object")
	}

	hexSSID := s.hexSSID(request.Ssid)
	start := time.Now()

	// Configure a service for the hidden SSID as a result of manual input SSID.
	if request.Hidden {
		props := map[string]interface{}{
			shillconst.ServicePropertyType:           shillconst.TypeWifi,
			shillconst.ServicePropertyWiFiHexSSID:    hexSSID,
			shillconst.ServicePropertyWiFiHiddenSSID: request.Hidden,
			shillconst.ServicePropertySecurityClass:  request.Security,
		}
		if _, err := m.ConfigureService(ctx, props); err != nil {
			return nil, errors.Wrap(err, "failed to configure a hidden SSID")
		}
	}
	props := map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID:   hexSSID,
		shillconst.ServicePropertySecurityClass: request.Security,
	}

	service, err := s.discoverService(ctx, m, props)
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover service")
	}
	discoveryTime := time.Since(start)

	shillProps, err := protoutil.DecodeFromShillValMap(request.Shillprops)
	if err != nil {
		return nil, err
	}
	for k, v := range shillProps {
		if err = service.SetProperty(ctx, k, v); err != nil {
			return nil, errors.Wrapf(err, "failed to set properties %s to %v", k, v)
		}
	}

	assocTime, configTime, err := s.connectService(ctx, service)
	if err != nil {
		return nil, err
	}

	return &wifi.ConnectResponse{
		ServicePath:       string(service.ObjectPath()),
		DiscoveryTime:     discoveryTime.Nanoseconds(),
		AssociationTime:   assocTime.Nanoseconds(),
		ConfigurationTime: configTime.Nanoseconds(),
	}, nil
}

func (s *ShillService) selectedService(ctx context.Context) (dbus.ObjectPath, error) {
	ctx, st := timing.Start(ctx, "wifi_service.selectedService")
	defer st.End()

	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return "", err
	}
	prop, err := dev.GetProperties(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get WiFi device properties")
	}
	servicePath, err := prop.GetObjectPath(shillconst.DevicePropertySelectedService)
	if err != nil {
		return "", errors.Wrap(err, "failed to get SelectedService")
	}

	return servicePath, nil
}

// SelectedService returns the object path of selected service of WiFi service.
func (s *ShillService) SelectedService(ctx context.Context, _ *empty.Empty) (*wifi.SelectedServiceResponse, error) {
	servicePath, err := s.selectedService(ctx)
	if err != nil {
		return nil, err
	}
	// Handle a special case of no selected service.
	// See: https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/shill/doc/device-api.txt
	if servicePath == "/" {
		return nil, errors.New("no selected service")
	}

	return &wifi.SelectedServiceResponse{
		ServicePath: string(servicePath),
	}, nil
}

// Disconnect disconnects from a WiFi service.
// This is the implementation of wifi.ShillService/Disconnect gRPC.
func (s *ShillService) Disconnect(ctx context.Context, request *wifi.DisconnectRequest) (ret *empty.Empty, retErr error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	ctx, st := timing.Start(ctx, "wifi_service.Disconnect")
	defer st.End()

	service, err := shill.NewService(ctx, dbus.ObjectPath(request.ServicePath))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}
	defer func() {
		// Try to remove profile even if Disconnect failed.
		if !request.RemoveProfile {
			return
		}
		if err := service.Remove(ctx); err != nil {
			if retErr != nil {
				testing.ContextLogf(ctx, "Failed to remove service profile of %v: %v", service, err)
			} else {
				ret = nil
				retErr = errors.Wrapf(err, "failed to remove service profile of %v", service)
			}
		}
	}()

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
	if err := pw.Expect(timeoutCtx, shillconst.ServicePropertyState, shillconst.ServiceStateIdle); err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "Disconnected")

	return &empty.Empty{}, nil
}

// AssureDisconnect assures that the WiFi service has disconnected within request.Timeout.
// It waits for the service state to be idle.
// This is the implementation of wifi.ShillService/AssureDisconnect gRPC.
func (s *ShillService) AssureDisconnect(ctx context.Context, request *wifi.AssureDisconnectRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	ctx, st := timing.Start(ctx, "wifi_service.AssureDisconnect")
	defer st.End()

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

	props, err := service.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service properties")
	}

	state, err := props.GetString(shillconst.ServicePropertyState)
	if err != nil {
		return nil, err
	}

	if state != shillconst.ServiceStateIdle {
		testing.ContextLog(ctx, "Wait for the service to be idle")
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(request.Timeout))
		defer cancel()

		if err := pw.Expect(timeoutCtx, shillconst.ServicePropertyState, shillconst.ServiceStateIdle); err != nil {
			return nil, err
		}
	}

	testing.ContextLog(ctx, "Disconnected")
	return &empty.Empty{}, nil
}

// uint16sToUint32s converts []uint16 to []uint32.
func uint16sToUint32s(s []uint16) []uint32 {
	ret := make([]uint32, len(s))
	for i, v := range s {
		ret[i] = uint32(v)
	}
	return ret
}

// QueryService queries shill service information.
// This is the implementation of wifi.ShillService/QueryService gRPC.
func (s *ShillService) QueryService(ctx context.Context, req *wifi.QueryServiceRequest) (*wifi.QueryServiceResponse, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	ctx, st := timing.Start(ctx, "wifi_service.QueryService")
	defer st.End()

	service, err := shill.NewService(ctx, dbus.ObjectPath(req.Path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}
	props, err := service.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service properties")
	}

	name, err := props.GetString(shillconst.ServicePropertyName)
	if err != nil {
		return nil, err
	}
	device, err := props.GetObjectPath(shillconst.ServicePropertyDevice)
	if err != nil {
		return nil, err
	}
	serviceType, err := props.GetString(shillconst.ServicePropertyType)
	if err != nil {
		return nil, err
	}
	mode, err := props.GetString(shillconst.ServicePropertyMode)
	if err != nil {
		return nil, err
	}
	state, err := props.GetString(shillconst.ServicePropertyState)
	if err != nil {
		return nil, err
	}
	visible, err := props.GetBool(shillconst.ServicePropertyVisible)
	if err != nil {
		return nil, err
	}
	isConnected, err := props.GetBool(shillconst.ServicePropertyIsConnected)
	if err != nil {
		return nil, err
	}

	bssid, err := props.GetString(shillconst.ServicePropertyWiFiBSSID)
	if err != nil {
		return nil, err
	}

	frequency, err := props.GetUint16(shillconst.ServicePropertyWiFiFrequency)
	if err != nil {
		return nil, err
	}
	frequencyList, err := props.GetUint16s(shillconst.ServicePropertyWiFiFrequencyList)
	if err != nil {
		return nil, err
	}
	hexSSID, err := props.GetString(shillconst.ServicePropertyWiFiHexSSID)
	if err != nil {
		return nil, err
	}
	hiddenSSID, err := props.GetBool(shillconst.ServicePropertyWiFiHiddenSSID)
	if err != nil {
		return nil, err
	}
	phyMode, err := props.GetUint16(shillconst.ServicePropertyWiFiPhyMode)
	if err != nil {
		return nil, err
	}
	guid, err := props.GetString(shillconst.ServicePropertyGUID)
	if err != nil {
		return nil, err
	}

	return &wifi.QueryServiceResponse{
		Name:        name,
		Device:      string(device),
		Type:        serviceType,
		Mode:        mode,
		State:       state,
		Visible:     visible,
		IsConnected: isConnected,
		Guid:        guid,
		Wifi: &wifi.QueryServiceResponse_Wifi{
			Bssid:         bssid,
			Frequency:     uint32(frequency),
			FrequencyList: uint16sToUint32s(frequencyList),
			HexSsid:       hexSSID,
			HiddenSsid:    hiddenSSID,
			PhyMode:       uint32(phyMode),
		},
	}, nil
}

// DeleteEntriesForSSID deletes all WiFi profile entries for a given SSID.
func (s *ShillService) DeleteEntriesForSSID(ctx context.Context, request *wifi.DeleteEntriesForSSIDRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	ctx, st := timing.Start(ctx, "wifi_service.DeleteEntriesForSSID")
	defer st.End()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	filter := map[string]interface{}{
		shillconst.ServicePropertyWiFiHexSSID: s.hexSSID(request.Ssid),
		shillconst.ProfileEntryPropertyType:   shillconst.TypeWifi,
	}
	if err := s.removeMatchedEntries(ctx, m, filter); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// cleanProfiles pops and removes all active profiles until default profile and
// then removes the WiFi test profile if still exists.
func (s *ShillService) cleanProfiles(ctx context.Context, m *shill.Manager) error {
	for {
		profile, err := m.ActiveProfile(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get active profile")
		}
		props, err := profile.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get properties from profile object")
		}
		name, err := props.GetString(shillconst.ProfilePropertyName)
		if name == shillconst.DefaultProfileName {
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
func (s *ShillService) removeWifiEntries(ctx context.Context, m *shill.Manager) error {
	filter := map[string]interface{}{
		shillconst.ProfileEntryPropertyType: shillconst.TypeWifi,
	}
	return s.removeMatchedEntries(ctx, m, filter)
}

// removeMatchedEntries traverses all profiles and removes all entries matching the properties in propFilter.
func (s *ShillService) removeMatchedEntries(ctx context.Context, m *shill.Manager, propFilter map[string]interface{}) error {
	profiles, err := m.Profiles(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get profiles")
	}
	for _, p := range profiles {
		props, err := p.GetProperties(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get properties from profile object")
		}
		entryIDs, err := props.GetStrings(shillconst.ProfilePropertyEntries)
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

// GetInterface returns the WiFi device interface name (e.g., wlan0).
func (s *ShillService) GetInterface(ctx context.Context, e *empty.Empty) (*wifi.GetInterfaceResponse, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager proxy")
	}
	netIf, err := shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi interface")
	}
	return &wifi.GetInterfaceResponse{
		Name: netIf,
	}, nil
}

// GetIPv4Addrs returns the IPv4 addresses for the network interface.
func (s *ShillService) GetIPv4Addrs(ctx context.Context, iface *wifi.GetIPv4AddrsRequest) (*wifi.GetIPv4AddrsResponse, error) {
	ifaceObj, err := net.InterfaceByName(iface.InterfaceName)
	if err != nil {
		return nil, err
	}

	addrs, err := ifaceObj.Addrs()
	if err != nil {
		return nil, err
	}

	var ret wifi.GetIPv4AddrsResponse

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			ret.Ipv4 = append(ret.Ipv4, ipnet.String())
		}
	}

	return &ret, nil
}

// GetHardwareAddr returns the HardwareAddr for the network interface.
func (s *ShillService) GetHardwareAddr(ctx context.Context, iface *wifi.GetHardwareAddrRequest) (*wifi.GetHardwareAddrResponse, error) {
	ifaceObj, err := net.InterfaceByName(iface.InterfaceName)
	if err != nil {
		return nil, err
	}

	return &wifi.GetHardwareAddrResponse{HwAddr: ifaceObj.HardwareAddr.String()}, nil
}

// RequestScans requests shill to trigger active scans on WiFi devices,
// and waits until at least req.Count scans are done.
func (s *ShillService) RequestScans(ctx context.Context, req *wifi.RequestScansRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	// Create watcher for ScanDone signal.
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager")
	}
	ifaceName, err := shill.WifiInterface(ctx, m, 10*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get WiFi interface")
	}
	supplicant, err := wpasupplicant.NewSupplicant(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to wpa_supplicant")
	}
	iface, err := supplicant.GetInterface(ctx, ifaceName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interface object paths")
	}
	sw, err := iface.DBusObject().CreateWatcher(ctx, wpasupplicant.DBusInterfaceSignalScanDone)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create signal watcher")
	}
	defer sw.Close(ctx)

	// Start background routine to wait for ScanDone signals.
	done := make(chan error, 1)
	// Wait for bg routine to end.
	defer func() { <-done }()
	// Notify the bg routine to end if we return early.
	bgCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func(ctx context.Context) {
		defer close(done)
		done <- func() error {
			count := int32(0)
			for count < req.Count {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case sig := <-sw.Signals:
					if success, err := iface.ParseScanDoneSignal(ctx, sig); err != nil {
						testing.ContextLogf(ctx, "Unexpected ScanDone signal %v: %v", sig, err)
					} else if !success {
						testing.ContextLog(ctx, "Unexpected ScanDone signal with failed scan")
					} else {
						count++
					}
				}
			}
			return nil
		}()
	}(bgCtx)

	// Trigger request scan every 200ms if the expected number of ScanDone is
	// not yet captured by the background routine and context deadline is not
	// yet reached.
	// It might be spammy, but shill handles it for us.
	for {
		if err := m.RequestScan(ctx, shill.TechnologyWifi); err != nil {
			return nil, err
		}
		select {
		case err := <-done:
			if err != nil {
				return nil, err
			}
			return &empty.Empty{}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// RequestRoam requests shill to roam to another BSSID and waits until the DUT has roamed.
// This is the implementation of wifi.ShillService/RequestRoam gRPC.
func (s *ShillService) RequestRoam(ctx context.Context, req *wifi.RequestRoamRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, err
	}

	dev, err := m.WaitForDeviceByName(ctx, req.InterfaceName, 5*time.Second)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find the device for the interface %s", req.InterfaceName)
	}

	if err := dev.RequestRoam(ctx, req.Bssid); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// Reassociate triggers reassociation with the current AP and waits until it has reconnected or the timeout expires.
// This is the implementation of wifi.WiFi/Reassociate gRPC.
func (s *ShillService) Reassociate(ctx context.Context, req *wifi.ReassociateRequest) (*empty.Empty, error) {
	ctx, cancelRet := reserveForReturn(ctx)
	defer cancelRet()

	supplicant, err := wpasupplicant.NewSupplicant(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to wpa_supplicant")
	}
	iface, err := supplicant.GetInterface(ctx, req.InterfaceName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interface object paths")
	}

	// Create watcher for PropertiesChanged signal.
	sw, err := iface.DBusObject().CreateWatcher(ctx, wpasupplicant.DBusInterfaceSignalPropertiesChanged)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create signal watcher")
	}
	defer sw.Close(ctx)

	// Trigger reassociation back to the current BSS.
	if err := iface.Reattach(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to call Reattach method")
	}

	// Watch the PropertiesChanged signals looking for state transitions which
	// indicate reassociation taking place. Most of the time on most platforms
	// this looks like:
	//
	//    State="associating"
	//    State="associated" or State="completed"
	//
	// But sometimes on specific platforms this can instead look like:
	//
	//    Scanning=true
	//    Scanning=false
	//    State="completed"
	//
	// So the detection logic here will accept either Scanning=true or
	// (State!="associated" && State!="completed") as indicating that we have
	// begun reassociation.
	associating := false
	ctx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout))
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx.Err(), "did not reassociate in time")
		case sig := <-sw.Signals:
			props := sig.Body[0].(map[string]dbus.Variant)
			testing.ContextLog(ctx, "PropertiesChanged: ", props)
			if val, ok := props["Scanning"]; ok {
				scanning := val.Value().(bool)
				if !associating && scanning {
					associating = true
				}
			}
			if val, ok := props["State"]; ok {
				state := val.Value().(string)
				associated := state == wpasupplicant.DBusInterfaceStateAssociated || state == wpasupplicant.DBusInterfaceStateCompleted
				if !associating && !associated {
					associating = true
				}
				if associating && associated {
					return &empty.Empty{}, nil
				}
			}
		}
	}
}

// MACRandomizeSupport tells if MAC randomization is supported for the WiFi device.
func (s *ShillService) MACRandomizeSupport(ctx context.Context, _ *empty.Empty) (*wifi.MACRandomizeSupportResponse, error) {
	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}

	prop, err := dev.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get WiFi device properties")
	}

	supported, err := prop.GetBool(shillconst.DevicePropertyMACAddrRandomSupported)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get WiFi device boolean prop %q",
			shillconst.DevicePropertyMACAddrRandomSupported)
	}
	return &wifi.MACRandomizeSupportResponse{Supported: supported}, nil
}

// GetMACRandomize tells if MAC randomization is enabled for the WiFi device.
func (s *ShillService) GetMACRandomize(ctx context.Context, _ *empty.Empty) (*wifi.GetMACRandomizeResponse, error) {
	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}

	prop, err := dev.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get WiFi device properties")
	}

	enabled, err := prop.GetBool(shillconst.DevicePropertyMACAddrRandomEnabled)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get WiFi device boolean prop %q",
			shillconst.DevicePropertyMACAddrRandomEnabled)
	}

	return &wifi.GetMACRandomizeResponse{Enabled: enabled}, nil
}

// SetMACRandomize sets the MAC randomization setting on the WiFi device.
// The original setting is returned for ease of restoring.
func (s *ShillService) SetMACRandomize(ctx context.Context, req *wifi.SetMACRandomizeRequest) (*wifi.SetMACRandomizeResponse, error) {
	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}

	prop, err := dev.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get WiFi device properties")
	}

	// Check if it is supported.
	support, err := prop.GetBool(shillconst.DevicePropertyMACAddrRandomSupported)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get WiFi device boolean prop %q",
			shillconst.DevicePropertyMACAddrRandomSupported)
	}
	if !support {
		return nil, errors.New("MAC randomization not supported")
	}
	// Get old setting.
	old, err := prop.GetBool(shillconst.DevicePropertyMACAddrRandomEnabled)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get WiFi device boolean prop %q",
			shillconst.DevicePropertyMACAddrRandomEnabled)
	}

	// NOP if the setting is already as requested.
	if req.Enable != old {
		if err := dev.SetProperty(ctx, shillconst.DevicePropertyMACAddrRandomEnabled, req.Enable); err != nil {
			return nil, errors.Wrapf(err, "failed to set WiFi device property %q",
				shillconst.DevicePropertyMACAddrRandomEnabled)
		}
	}

	return &wifi.SetMACRandomizeResponse{OldSetting: old}, nil
}

// WaitScanIdle waits for not scanning state. If there's a running scan, it
// waits for the scan to be done with timeout 10 seconds.
// This is useful when the test sets some parameters regarding scans and wants
// to avoid noises due to in-progress scans.
func (s *ShillService) WaitScanIdle(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}

	pw, err := dev.CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	// Check initial state.
	prop, err := dev.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get properties of WiFi device")
	}
	scanning, err := prop.GetBool(shillconst.DevicePropertyScanning)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get WiFi scanning state")
	}
	if !scanning {
		// Already in the expected state, return immediately.
		return &empty.Empty{}, nil
	}

	// Wait scanning to become false.
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pw.Expect(timeoutCtx, shillconst.DevicePropertyScanning, false); err != nil {
		return nil, errors.Wrap(err, "failed to wait for not scanning state")
	}

	return &empty.Empty{}, nil
}

// hexSSID converts a SSID into the format of WiFi.HexSSID in shill.
// As in our tests, the SSID might contain non-ASCII characters, use WiFi.HexSSID
// field for better compatibility.
// Note: shill has the hex in upper case.
func (s *ShillService) hexSSID(ssid []byte) string {
	return strings.ToUpper(hex.EncodeToString(ssid))
}

// uint16sEqualUint32s returns true if a is equal to b.
func uint16sEqualUint32s(a []uint16, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })

	for i, v := range b {
		if v != uint32(a[i]) {
			return false
		}
	}
	return true
}

// ExpectWifiFrequencies checks if the device discovers the given SSID on the specific frequencies.
func (s *ShillService) ExpectWifiFrequencies(ctx context.Context, req *wifi.ExpectWifiFrequenciesRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	ctx, st := timing.Start(ctx, "wifi_service.ExpectWifiFrequencies")
	defer st.End()
	testing.ContextLog(ctx, "ExpectWifiFrequencies: ", req)

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a manager object")
	}

	hexSSID := s.hexSSID(req.Ssid)
	query := map[string]interface{}{
		shillconst.ServicePropertyType:        shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID: hexSSID,
	}

	service, err := s.discoverService(ctx, m, query)
	if err != nil {
		return nil, err
	}

	// Spawn watcher for checking property change.
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	shortCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	for {
		props, err := service.GetProperties(shortCtx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get service properties")
		}
		freqs, err := props.GetUint16s(shillconst.ServicePropertyWiFiFrequencyList)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get property %s", shillconst.ServicePropertyWiFiFrequencyList)
		}
		if uint16sEqualUint32s(freqs, req.Frequencies) {
			testing.ContextLogf(shortCtx, "Got wanted frequencies %v for service with SSID: %s", req.Frequencies, req.Ssid)
			return &empty.Empty{}, nil
		}

		testing.ContextLogf(shortCtx, "Got frequencies %v for service with SSID: %s; want %v; waiting for update", freqs, req.Ssid, req.Frequencies)
		if _, err := pw.WaitAll(shortCtx, shillconst.ServicePropertyWiFiFrequencyList); err != nil {
			return nil, errors.Wrap(err, "failed to wait for the service property change")
		}
	}
}

func (s *ShillService) wifiDev(ctx context.Context) (*shill.Manager, *shill.Device, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create Manager object")
	}
	iface, err := shill.WifiInterface(ctx, m, 5*time.Second)
	if err != nil {
		return m, nil, errors.Wrap(err, "failed to get a WiFi device")
	}
	dev, err := m.DeviceByName(ctx, iface)
	if err != nil {
		return m, nil, errors.Wrapf(err, "failed to find the device for interface %s", iface)
	}
	return m, dev, nil
}

func (s *ShillService) GetBgscanConfig(ctx context.Context, e *empty.Empty) (*wifi.GetBgscanConfigResponse, error) {
	ctx, st := timing.Start(ctx, "wifi_service.GetBgscan")
	defer st.End()

	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}

	props, err := dev.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi device properties")
	}
	method, err := props.GetString(shillconst.DevicePropertyWiFiBgscanMethod)
	if err != nil {
		return nil, err
	}
	interval, err := props.GetUint16(shillconst.DevicePropertyWiFiScanInterval)
	if err != nil {
		return nil, err
	}
	shortInterval, err := props.GetUint16(shillconst.DevicePropertyWiFiBgscanShortInterval)
	if err != nil {
		return nil, err
	}
	return &wifi.GetBgscanConfigResponse{
		Config: &wifi.BgscanConfig{
			Method:        method,
			LongInterval:  uint32(interval),
			ShortInterval: uint32(shortInterval),
		},
	}, nil
}

func (s *ShillService) SetBgscanConfig(ctx context.Context, req *wifi.SetBgscanConfigRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	ctx, st := timing.Start(ctx, "wifi_service.SetBgscan")
	defer st.End()

	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}

	setProp := func(ctx context.Context, key string, val interface{}) error {
		if err := dev.SetProperty(ctx, key, val); err != nil {
			return errors.Wrapf(err, "failed to set the WiFi device property %s with value %v", key, val)
		}
		return nil
	}

	if err := setProp(ctx, shillconst.DevicePropertyWiFiBgscanMethod, req.Config.Method); err != nil {
		return nil, err
	}
	if err := setProp(ctx, shillconst.DevicePropertyWiFiScanInterval, uint16(req.Config.LongInterval)); err != nil {
		return nil, err
	}
	if err := setProp(ctx, shillconst.DevicePropertyWiFiBgscanShortInterval, uint16(req.Config.ShortInterval)); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// DisableEnableTest disables and then enables the WiFi interface. This is the main body of the DisableEnable test.
// It first disables the WiFi interface and waits for the idle state; then waits for the IsConnected property after enable.
// The reason we place most of the logic here is that, we need to spawn a shill properties watcher before disabling/enabling
// the WiFi interface, so we won't lose the state change events between the gRPC commands of disabling/enabling interface
// and checking state.
func (s *ShillService) DisableEnableTest(ctx context.Context, request *wifi.DisableEnableTestRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	dev, err := m.DeviceByName(ctx, request.InterfaceName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find the device for interface %s", request.InterfaceName)
	}

	// Spawn watcher before disabling and enabling.
	service, err := shill.NewService(ctx, dbus.ObjectPath(request.ServicePath))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	// Form a closure here so we can ensure that interface will be re-enabled even if something failed.
	if err := func() (retErr error) {
		// Disable WiFi interface.
		testing.ContextLog(ctx, "Disabling WiFi interface: ", request.InterfaceName)
		if err := dev.Disable(ctx); err != nil {
			return errors.Wrap(err, "failed to disable WiFi device")
		}

		defer func() {
			// Re-enable WiFi interface.
			testing.ContextLog(ctx, "Enabling WiFi interface: ", request.InterfaceName)
			if err := dev.Enable(ctx); err != nil {
				if retErr == nil {
					retErr = errors.Wrap(err, "failed to enable WiFi device")
				} else {
					testing.ContextLog(ctx, "Failed to enable WiFi device and the test is already failed: ", err)
				}
			}
		}()

		// Wait for WiFi service becomes idle state.
		testing.ContextLog(ctx, "Waiting for idle state")
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := pw.Expect(timeoutCtx, shillconst.ServicePropertyState, shillconst.ServiceStateIdle); err != nil {
			return errors.Wrap(err, "failed to wait for idle state after disabling")
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	// The interface has been re-enabled as a defer statement in the anonymous function above,
	// now just need to wait for WiFi service becomes connected.
	testing.ContextLog(ctx, "Waiting for IsConnected property")
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := pw.Expect(timeoutCtx, shillconst.ServicePropertyIsConnected, true); err != nil {
		return nil, errors.Wrap(err, "failed to wait for IsConnected property after enabling")
	}

	return &empty.Empty{}, nil
}

// ConfigureAndAssertAutoConnect configures the matched shill service and then waits for the IsConnected property becomes true.
// Note that this function does not attempt to connect; it waits for auto connect instead.
func (s *ShillService) ConfigureAndAssertAutoConnect(ctx context.Context,
	request *wifi.ConfigureAndAssertAutoConnectRequest) (*wifi.ConfigureAndAssertAutoConnectResponse, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a manager object")
	}

	props, err := protoutil.DecodeFromShillValMap(request.Props)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode shill properties")
	}
	servicePath, err := m.ConfigureService(ctx, props)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure service")
	}
	testing.ContextLog(ctx, "Configured service; start scanning")

	service, err := shill.NewService(ctx, servicePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create service object")
	}
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create property watcher")
	}
	defer pw.Close(ctx)

	// Service may become connected between ConfigureService and CreateWatcher, we would lose the property changing event of IsConnected then.
	// Checking once after creating watcher should be good enough since we expect that the connection should not disconnect without our attempt.
	p, err := service.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	isConnected, err := p.GetBool(shillconst.ServicePropertyIsConnected)
	if err != nil {
		return nil, err
	}
	if isConnected {
		return &wifi.ConfigureAndAssertAutoConnectResponse{Path: string(servicePath)}, nil
	}

	done := make(chan error, 1)
	// Wait for bg routine to end.
	defer func() { <-done }()
	// Notify the bg routine to end if we return early.
	bgCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func(ctx context.Context) {
		defer close(done)
		done <- pw.Expect(ctx, shillconst.ServicePropertyIsConnected, true)
	}(bgCtx)

	// Request a scan every 200ms until the background routine catches the IsConnected signal.
	for {
		if err := m.RequestScan(ctx, shill.TechnologyWifi); err != nil {
			return nil, errors.Wrap(err, "failed to request scan")
		}
		select {
		case err := <-done:
			if err != nil {
				return nil, errors.Wrap(err, "failed to wait for IsConnected property becoming true")
			}
			return &wifi.ConfigureAndAssertAutoConnectResponse{Path: string(servicePath)}, nil
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// GetCurrentTime returns the current local time in the given format.
func (s *ShillService) GetCurrentTime(ctx context.Context, _ *empty.Empty) (*wifi.GetCurrentTimeResponse, error) {
	now := time.Now()
	return &wifi.GetCurrentTimeResponse{NowSecond: now.Unix(), NowNanosecond: int64(now.Nanosecond())}, nil
}

// ExpectShillProperty is a streaming gRPC, takes a shill service path, expects a list of property
// criteria in order, and takes a list of shill properties to monitor. When a property's value is
// expected, it responds the property's (key, value) pair. The method sends an empty response as the
// property watcher is set. A property matching criterion consists of a property name, a list of
// expected values, a list of excluded values, and a "CheckType". We say a criterion is met iff the
// property value is in one of the expected values and not in any of the excluded values. If the
// property value is one of the excluded values, the method fails immediately. The call monitors the
// specified shill properties and returns the monitor results as a chronological list of pairs
// (changed property, changed value) at the end.
// For CheckMethod, it has three methods:
// 1. CHECK_ONLY: checks if the criterion is met.
// 2. ON_CHANGE: waits for the property changes to the expected values.
// 3. CHECK_WAIT: checks if the criterion is met; if not, waits until the property's value is met.
// This is the implementation of wifi.ShillService/ExpectShillProperty gRPC.
func (s *ShillService) ExpectShillProperty(req *wifi.ExpectShillPropertyRequest, sender wifi.ShillService_ExpectShillPropertyServer) error {
	ctx, cancel := reserveForReturn(sender.Context())
	defer cancel()

	service, err := shill.NewService(ctx, dbus.ObjectPath(req.ObjectPath))
	if err != nil {
		return errors.Wrap(err, "failed to create a service object")
	}
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a watcher")
	}
	defer pw.Close(ctx)

	if err := sender.Send(&wifi.ExpectShillPropertyResponse{}); err != nil {
		return errors.Wrap(err, "failed to send a response")
	}

	// foundIn returns true if the property value v is found in vs; false otherwise.
	foundIn := func(v interface{}, vs []interface{}) bool {
		for _, ev := range vs {
			// Protoutil does not support uint16 in the meantime.
			// Change the type of v to uint32, if its type is uint16.
			if x, ok := v.(uint16); ok {
				v = uint32(x)
			}
			if reflect.DeepEqual(ev, v) {
				return true
			}
		}
		return false
	}

	var monitorResult []protoutil.ShillPropertyHolder
	for _, p := range req.Props {
		var expectedVals []interface{}
		for _, sv := range p.AnyOf {
			v, err := protoutil.FromShillVal(sv)
			if err != nil {
				return err
			}
			expectedVals = append(expectedVals, v)
		}

		var excludedVals []interface{}
		for _, sv := range p.NoneOf {
			v, err := protoutil.FromShillVal(sv)
			if err != nil {
				return err
			}
			excludedVals = append(excludedVals, v)
		}

		// Check the current value of the property.
		if p.Method != wifi.ExpectShillPropertyRequest_ON_CHANGE {
			props, err := service.GetProperties(ctx)
			if err != nil {
				return err
			}

			val, err := props.Get(p.Key)
			if err != nil {
				return err
			}

			if foundIn(val, excludedVals) {
				return errors.Errorf("unexpected property [ %q ] value: got %s, want any of %v", p.Key, val, expectedVals)
			}

			if foundIn(val, expectedVals) {
				shillVal, err := protoutil.ToShillVal(val)
				if err != nil {
					return err
				}
				if err := sender.Send(&wifi.ExpectShillPropertyResponse{Key: p.Key, Val: shillVal}); err != nil {
					return errors.Wrap(err, "failed to send response")
				}

				// Skip waiting for the property change and move to the next criterion.
				continue
			}

			// Return an error if the method is CHECK_ONLY and the property does not meet the criterion.
			if p.Method == wifi.ExpectShillPropertyRequest_CHECK_ONLY {
				return errors.Errorf("unexpected property [ %q ] value: got %s, want any of %v", p.Key, val, expectedVals)
			}
		}

		// Wait for the property to change to an expected or unexpected value. Record the property changes we are monitoring.
		var propVal interface{}
		for {
			prop, val, _, err := pw.Wait(ctx)
			if err != nil {
				return errors.Wrapf(err, "failed to wait for the property %s", prop)
			}
			for _, mp := range req.MonitorProps {
				if mp == prop {
					monitorResult = append(monitorResult, protoutil.ShillPropertyHolder{Name: prop, Value: val})
					break
				}
			}
			if prop == p.Key {
				if foundIn(val, expectedVals) {
					propVal = val
					break
				}
				if foundIn(val, excludedVals) {
					return errors.Errorf("unexpected property %q: got %s, want any of %v", prop, val, expectedVals)
				}
			}
		}

		shillVal, err := protoutil.ToShillVal(propVal)
		if err != nil {
			return err
		}

		if err := sender.Send(&wifi.ExpectShillPropertyResponse{Key: p.Key, Val: shillVal}); err != nil {
			return errors.Wrap(err, "failed to send response")
		}
	}

	rslt, err := protoutil.EncodeToShillPropertyChangedSignalList(monitorResult)
	if err != nil {
		return err
	}

	if err := sender.Send(&wifi.ExpectShillPropertyResponse{Props: rslt, MonitorDone: true}); err != nil {
		return errors.Wrap(err, "failed to send response")
	}

	return nil
}

// expectServiceProperty sets up a properties watcher before calling f, and waits for the given property/value.
func (*ShillService) expectServiceProperty(ctx context.Context, service *shill.Service, prop string, val interface{}, f func() error) (dbus.Sequence, error) {
	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create a service property watcher")
	}
	defer pw.Close(ctx)
	if err := f(); err != nil {
		return 0, err
	}
	for {
		p, v, s, err := pw.Wait(ctx)
		if err != nil {
			return 0, errors.Wrap(err, "failed to wait for property")
		}
		if p == prop && reflect.DeepEqual(v, val) {
			return s, nil
		}
	}
}

// ProfileBasicTest is the main body of the ProfileBasic test, which creates, pushes, and pops the profiles and asserts the connection states between those operations.
// This is the implementation of wifi.ShillService/ProfileBasicTest gRPC.
func (s *ShillService) ProfileBasicTest(ctx context.Context, req *wifi.ProfileBasicTestRequest) (_ *empty.Empty, retErr error) {
	const (
		profileBottomName = "bottom"
		profileTopName    = "top"

		pingLossThreshold = 20.0
	)

	expectIdle := func(ctx context.Context, service *shill.Service, f func() error) (dbus.Sequence, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return s.expectServiceProperty(ctx, service, shillconst.ServicePropertyState, shillconst.ServiceStateIdle, f)
	}
	expectIsConnected := func(ctx context.Context, service *shill.Service, f func() error) (dbus.Sequence, error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return s.expectServiceProperty(ctx, service, shillconst.ServicePropertyIsConnected, true, f)
	}
	connectAndPing := func(ctx context.Context, service *shill.Service, ap *wifi.ProfileBasicTestRequest_Config) error {
		shillProps, err := protoutil.DecodeFromShillValMap(ap.ShillProps)
		if err != nil {
			return err
		}
		for k, v := range shillProps {
			if err = service.SetProperty(ctx, k, v); err != nil {
				return errors.Wrapf(err, "failed to set property %s to %v", k, v)
			}
		}
		if _, _, err := s.connectService(ctx, service); err != nil {
			return err
		}

		res, err := local_ping.NewLocalRunner().Ping(ctx, ap.Ip)
		if err != nil {
			return err
		}
		if res.Loss > pingLossThreshold {
			return errors.Errorf("unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
		}

		return nil
	}

	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a shill manager")
	}

	service0, err := s.discoverService(ctx, m, map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID:   s.hexSSID(req.Ap0.Ssid),
		shillconst.ServicePropertySecurityClass: req.Ap0.Security,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover AP0")
	}
	service1, err := s.discoverService(ctx, m, map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyWiFiHexSSID:   s.hexSSID(req.Ap1.Ssid),
		shillconst.ServicePropertySecurityClass: req.Ap1.Security,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover AP1")
	}

	if _, err := m.CreateProfile(ctx, profileBottomName); err != nil {
		return nil, errors.Wrapf(err, "failed to create profile %q", profileBottomName)
	}
	defer func(ctx context.Context) {
		// Ignore the error of popping profile since it may be popped during test.
		m.PopProfile(ctx, profileBottomName)
		if err := m.RemoveProfile(ctx, profileBottomName); err != nil {
			if retErr != nil {
				testing.ContextLogf(ctx, "Failed to remove profile %q and the test has already failed: %v", profileBottomName, err)
			} else {
				retErr = errors.Wrapf(err, "failed to remove profile %q", profileBottomName)
			}
		}
	}(ctx)
	if _, err := m.PushProfile(ctx, profileBottomName); err != nil {
		return nil, errors.Wrapf(err, "failed to push profile %q", profileBottomName)
	}

	if err := connectAndPing(ctx, service0, req.Ap0); err != nil {
		return nil, err
	}

	// We should lose the credentials if we pop the profile.
	if _, err := expectIdle(ctx, service0, func() error {
		return m.PopProfile(ctx, profileBottomName)
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to pop profile %q and wait for idle", profileBottomName)
	}
	// We should retrieve the credentials if we push the profile back.
	if _, err := expectIsConnected(ctx, service0, func() error {
		_, err := m.PushProfile(ctx, profileBottomName)
		return err
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to push profile %q and wait for isConnected", profileBottomName)
	}

	// Explicitly disconnect from AP0.
	if _, err := expectIdle(ctx, service0, func() error {
		return service0.Disconnect(ctx)
	}); err != nil {
		return nil, errors.Wrap(err, "failed to disconnect from AP0")
	}

	if _, err := m.CreateProfile(ctx, profileTopName); err != nil {
		return nil, errors.Wrapf(err, "failed to create profile %q", profileTopName)
	}
	defer func(ctx context.Context) {
		// Ignore the error of popping profile since it may be popped during test.
		m.PopProfile(ctx, profileTopName)
		if err := m.RemoveProfile(ctx, profileTopName); err != nil {
			if retErr != nil {
				testing.ContextLogf(ctx, "Failed to remove profile %q and the test has already failed: %v", profileTopName, err)
			} else {
				retErr = errors.Wrapf(err, "failed to remove profile %q", profileTopName)
			}
		}
	}(ctx)

	// The modification of the profile stack should clear the "explicitly disconnected"
	// flag on all services and leads to a re-connecting.
	if _, err := expectIsConnected(ctx, service0, func() error {
		_, err := m.PushProfile(ctx, profileTopName)
		return err
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to push profile %q and wait for isConnected", profileTopName)
	}

	if err := connectAndPing(ctx, service1, req.Ap1); err != nil {
		return nil, err
	}

	// Removing the entries of AP1 should cause a disconnecting to AP1 and then a reconnecting to AP0.
	// Recording the sequence code of the Idle/IsConnected events helps us to determine the order.
	var idleSeq, isConnectedSeq dbus.Sequence
	if isConnectedSeq, err = expectIsConnected(ctx, service0, func() error {
		var innerErr error
		if idleSeq, innerErr = expectIdle(ctx, service1, func() error {
			return s.removeMatchedEntries(ctx, m, map[string]interface{}{
				shillconst.ServicePropertyWiFiHexSSID: s.hexSSID(req.Ap1.Ssid),
				shillconst.ProfileEntryPropertyType:   shillconst.TypeWifi,
			})
		}); innerErr != nil {
			return innerErr
		}
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to remove entries of AP0 and wait for reconnect")
	}
	if idleSeq > isConnectedSeq {
		return nil, errors.New("expected to get the Idle signal of AP0 before the IsConnected signal of AP1 but got an inverse order")
	}

	if err := connectAndPing(ctx, service1, req.Ap1); err != nil {
		return nil, err
	}

	// Popping the current profile should be similar to the case above.
	if isConnectedSeq, err = expectIsConnected(ctx, service0, func() error {
		var innerErr error
		if idleSeq, innerErr = expectIdle(ctx, service1, func() error {
			return m.PopProfile(ctx, profileTopName)
		}); innerErr != nil {
			return innerErr
		}
		return nil
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to pop profile %q and wait for reconnect", profileTopName)
	}
	if idleSeq > isConnectedSeq {
		return nil, errors.New("expected to get the Idle signal of AP0 before the IsConnected signal of AP1 but got an inverse order")
	}

	if _, err := m.PushProfile(ctx, profileTopName); err != nil {
		return nil, errors.Wrapf(err, "failed to push profile %q", profileTopName)
	}

	// Explicitly disconnect from AP0.
	if _, err := expectIdle(ctx, service0, func() error {
		return service0.Disconnect(ctx)
	}); err != nil {
		return nil, errors.Wrap(err, "failed to disconnect from AP0")
	}

	// Verify that popping a profile which does not affect the service should also clear the service's "explicitly disconnected" flag.
	if _, err := expectIsConnected(ctx, service0, func() error {
		return m.PopProfile(ctx, profileTopName)
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to pop profile %q and wait for isConnected", profileTopName)
	}
	return &empty.Empty{}, nil
}

// waitForWifiAvailable waits for WiFi to be available in shill.
func (s *ShillService) waitForWifiAvailable(ctx context.Context, m *shill.Manager) error {
	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)

	prop, err := m.GetProperties(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get Manager's properties from shill")
	}

	findWifi := func(list []string) bool {
		for _, s := range list {
			if s == shillconst.TypeWifi {
				return true
			}
		}
		return false
	}

	techs, err := prop.GetStrings(shillconst.ManagerPropertyAvailableTechnologies)
	if err != nil {
		return errors.Wrap(err, "failed to get availabe technologies property")
	}
	if findWifi(techs) {
		return nil
	}
	for {
		val, err := pw.WaitAll(ctx, shillconst.ManagerPropertyAvailableTechnologies)
		if err != nil {
			return errors.Wrap(err, "failed to wait available technologies changes")
		}
		techs, ok := val[0].([]string)
		if !ok {
			return errors.Errorf("unexpected available technologies value: %v", val)
		}
		if findWifi(techs) {
			return nil
		}
	}
}

// GetWifiEnabled checks to see if Wifi is an enabled technology on shill.
// This call will wait for WiFi to appear in available technologies so we
// can get correct enabled setting.
func (s *ShillService) GetWifiEnabled(ctx context.Context, _ *empty.Empty) (*wifi.GetWifiEnabledResponse, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}

	if err := s.waitForWifiAvailable(ctx, manager); err != nil {
		return nil, err
	}

	prop, err := manager.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Manager' properties from shill")
	}

	technologies, err := prop.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
	if err != nil {
		return nil, errors.Wrap(err, "failed go get enabled technologies property")
	}

	for _, t := range technologies {
		if t == string(shill.TechnologyWifi) {
			return &wifi.GetWifiEnabledResponse{Enabled: true}, nil
		}
	}
	return &wifi.GetWifiEnabledResponse{Enabled: false}, nil
}

// SetWifiEnabled persistently enables/disables Wifi via shill.
func (s *ShillService) SetWifiEnabled(ctx context.Context, request *wifi.SetWifiEnabledRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	_, err = shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi interface")
	}
	if request.Enabled {
		if err := manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
			return nil, errors.Wrap(err, "could not enable wifi via shill")
		}
		return &empty.Empty{}, nil
	}
	if err := manager.DisableTechnology(ctx, shill.TechnologyWifi); err != nil {
		return nil, errors.Wrap(err, "could not disable wifi via shill")
	}
	return &empty.Empty{}, nil
}

// SetDHCPProperties sets DHCP properties in shill and returns the original values.
func (s *ShillService) SetDHCPProperties(ctx context.Context, req *wifi.SetDHCPPropertiesRequest) (ret *wifi.SetDHCPPropertiesResponse, retErr error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a Manager object")
	}
	prop, err := m.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Manager's properties")
	}
	// We expect that the two properties below could be not there yet.
	// If that's the case, the default value is set as an empty string.
	oldHostname, err := prop.GetString(shillconst.DHCPPropertyHostname)
	if err != nil {
		oldHostname = ""
	}
	oldVendor, err := prop.GetString(shillconst.DHCPPropertyVendorClass)
	if err != nil {
		oldVendor = ""
	}

	// Revert the DHCP properties if something goes wrong.
	defer func(ctx context.Context) {
		if retErr == nil {
			// No need of restore.
			return
		}
		if err := m.SetProperty(ctx, shillconst.DHCPPropertyHostname, oldHostname); err != nil {
			testing.ContextLogf(ctx, "Failed to restore DHCP hostname to %q: %v", oldHostname, err)
		}
		if err := m.SetProperty(ctx, shillconst.DHCPPropertyVendorClass, oldVendor); err != nil {
			testing.ContextLogf(ctx, "Failed to restore DHCP vendor class to %q: %v", oldVendor, err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	hostname := req.Props.Hostname
	if oldHostname != hostname {
		if err := m.SetProperty(ctx, shillconst.DHCPPropertyHostname, hostname); err != nil {
			return nil, errors.Wrapf(err, "failed to set DHCP hostname to %q", hostname)
		}
	}
	vendor := req.Props.VendorClass
	if oldVendor != vendor {
		if err := m.SetProperty(ctx, shillconst.DHCPPropertyVendorClass, vendor); err != nil {
			return nil, errors.Wrapf(err, "failed to set DHCP vendor class to %q", vendor)
		}
	}

	return &wifi.SetDHCPPropertiesResponse{
		Props: &wifi.DHCPProperties{
			Hostname:    oldHostname,
			VendorClass: oldVendor,
		},
	}, nil
}

// WaitForBSSID waits for a specific BSSID to be found.
func (s *ShillService) WaitForBSSID(ctx context.Context, request *wifi.WaitForBSSIDRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager")
	}
	ifaceName, err := shill.WifiInterface(ctx, m, 10*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get WiFi interface")
	}
	supplicant, err := wpasupplicant.NewSupplicant(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to wpa_supplicant")
	}
	iface, err := supplicant.GetInterface(ctx, ifaceName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the wpa_supplicant's interface's object path")
	}
	bssid, err := net.ParseMAC(request.Bssid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse BSSID")
	}
	if err := s.waitForBSSID(ctx, iface, request.Ssid, bssid); err != nil {
		return nil, errors.Wrap(err, "failed to wait for BSSID")
	}
	return &empty.Empty{}, nil
}

// EAPAuthSkipped is a streaming gRPC, who watches wpa_supplicant's D-Bus signals until the next connection
// completes, and tells that the EAP authentication is skipped (i.e., PMKSA is cached and used) or not.
// Note that the method sends an empty response after the signal watcher is initialized.
func (s *ShillService) EAPAuthSkipped(_ *empty.Empty, sender wifi.ShillService_EAPAuthSkippedServer) error {
	ctx, cancel := reserveForReturn(sender.Context())
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create shill manager")
	}
	ifaceName, err := shill.WifiInterface(ctx, m, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to get WiFi interface")
	}
	supplicant, err := wpasupplicant.NewSupplicant(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to wpa_supplicant")
	}
	iface, err := supplicant.GetInterface(ctx, ifaceName)
	if err != nil {
		return errors.Wrap(err, "failed to get interface object paths")
	}

	sw, err := iface.DBusObject().CreateWatcher(ctx, wpasupplicant.DBusInterfaceSignalPropertiesChanged, wpasupplicant.DBusInterfaceSignalEAP)
	if err != nil {
		return errors.Wrap(err, "failed to create signal watcher")
	}
	defer sw.Close(ctx)

	// Send an empty response to notify that the watcher is ready.
	if err := sender.Send(&wifi.EAPAuthSkippedResponse{}); err != nil {
		return errors.Wrap(err, "failed to send a ready signal")
	}

	// Watch if there is any EAP signal until the connection completes.
	skipped := true
	for {
		var s *dbus.Signal
		select {
		case s = <-sw.Signals:
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "failed to wait for signal")
		}

		switch name := wpasupplicant.SignalName(s); name {
		case wpasupplicant.DBusInterfaceSignalEAP:
			// Any of the EAP signals indicate that wpa_supplicant has started an EAP authentication state machine.
			skipped = false
		case wpasupplicant.DBusInterfaceSignalPropertiesChanged:
			props := s.Body[0].(map[string]dbus.Variant)
			if val, ok := props["State"]; ok {
				if state := val.Value().(string); state == wpasupplicant.DBusInterfaceStateCompleted {
					return sender.Send(&wifi.EAPAuthSkippedResponse{Skipped: skipped})
				}
			}
		default:
			return errors.Errorf("unexpected name type: %s", name)
		}
	}
}

// DisconnectReason is a streaming gRPC, who waits for the wpa_supplicant's
// DisconnectReason property change, and returns the code to the client.
// To notify the caller that it is ready, it sends an empty response after
// the signal watcher is initialized.
// This is the implementation of wifi.ShillService/DisconnectReason gRPC.
func (s *ShillService) DisconnectReason(_ *empty.Empty, sender wifi.ShillService_DisconnectReasonServer) error {
	ctx, cancel := reserveForReturn(sender.Context())
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create shill manager")
	}
	ifaceName, err := shill.WifiInterface(ctx, m, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to get WiFi interface")
	}
	supplicant, err := wpasupplicant.NewSupplicant(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to wpa_supplicant")
	}
	iface, err := supplicant.GetInterface(ctx, ifaceName)
	if err != nil {
		return errors.Wrap(err, "failed to get interface object paths")
	}

	sw, err := iface.DBusObject().CreateWatcher(ctx, wpasupplicant.DBusInterfaceSignalPropertiesChanged)
	if err != nil {
		return errors.Wrap(err, "failed to create signal watcher")
	}
	defer sw.Close(ctx)

	// Send an empty response to notify that the watcher is ready.
	if err := sender.Send(&wifi.DisconnectReasonResponse{}); err != nil {
		return errors.Wrap(err, "failed to send a ready signal")
	}

	for {
		select {
		case s := <-sw.Signals:
			name := wpasupplicant.SignalName(s)
			if name != wpasupplicant.DBusInterfaceSignalPropertiesChanged {
				return errors.Errorf("unexpected name type: %s", name)
			}
			props := s.Body[0].(map[string]dbus.Variant)
			if val, ok := props[wpasupplicant.DBusInterfacePropDisconnectReason]; ok {
				reason := val.Value().(int32)
				return sender.Send(&wifi.DisconnectReasonResponse{Reason: reason})
			}
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "failed to wait for signal")
		}
	}
}

// suspend suspends the DUT for wakeUpTimeout. On success, the time of this
// suspend is returned.
// If checkEarlyWake is true, the call will fail when the suspend time is
// shorter than wakeUpTimeout.
// TODO(b/171280216): Extract these logics from network component.
func suspend(ctx context.Context, wakeUpTimeout time.Duration, checkEarlyWake bool) (time.Duration, error) {
	const (
		powerdDBusSuspendPath = "/usr/bin/powerd_dbus_suspend"
		rtcPath               = "/sys/class/rtc/rtc0/since_epoch"
		pauseEthernetHookPath = "/run/autotest_pause_ethernet_hook"
	)

	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to lock the check network hook")
	}
	defer unlock()

	rtcTimeSeconds := func() (int, error) {
		b, err := ioutil.ReadFile(rtcPath)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to read the %s", rtcPath)
		}
		return strconv.Atoi(strings.TrimSpace(string(b)))
	}

	if wakeUpTimeout < 2*time.Second {
		// May cause DUT not wake from sleep if the suspend time is 1 second.
		// It happens when the current clock (floating point) is close to the
		// next integer, as the RTC sysfs interface only accepts integers.
		// Make sure it is larger than or equal to 2.
		return 0, errors.Errorf("unexpected wake up timeout: got %s, want >= 2 seconds", wakeUpTimeout)
	}

	startRTC, err := rtcTimeSeconds()
	if err != nil {
		return 0, err
	}

	wakeUpTimeoutSecond := int(wakeUpTimeout.Seconds())
	if err := testexec.CommandContext(ctx, powerdDBusSuspendPath,
		"--delay=0", // By default it delays the start of suspending by a second.
		fmt.Sprintf("--wakeup_timeout=%d", wakeUpTimeoutSecond),  // Ask the powerd_dbus_suspend to spawn a RTC alarm to wake the DUT up after wakeUpTimeoutSecond.
		fmt.Sprintf("--suspend_for_sec=%d", wakeUpTimeoutSecond), // Request powerd daemon to suspend for wakeUpTimeoutSecond.
	).Run(); err != nil {
		return 0, err
	}

	finishRTC, err := rtcTimeSeconds()
	if err != nil {
		return 0, err
	}

	suspendedInterval := finishRTC - startRTC
	testing.ContextLogf(ctx, "RTC suspend time: %d", suspendedInterval)

	if checkEarlyWake && suspendedInterval < wakeUpTimeoutSecond {
		return 0, errors.Errorf("the DUT wakes up too early, got %d, want %d", suspendedInterval, wakeUpTimeoutSecond)
	}

	return time.Duration(suspendedInterval) * time.Second, nil
}

// SuspendAssertConnect suspends the DUT and waits for connection after resuming.
func (s *ShillService) SuspendAssertConnect(ctx context.Context, req *wifi.SuspendAssertConnectRequest) (*wifi.SuspendAssertConnectResponse, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	service, err := shill.NewService(ctx, dbus.ObjectPath(req.ServicePath))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a new shill service")
	}
	pw, err := service.CreateWatcher(ctx)
	defer pw.Close(ctx)

	if _, err := suspend(ctx, time.Duration(req.WakeUpTimeout), true /* checkEarlyWake */); err != nil {
		return nil, errors.Wrap(err, "failed to suspend")
	}

	resumeStartTime := time.Now()

	wCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	if err := pw.Expect(wCtx, shillconst.ServicePropertyState, shillconst.ServiceStateIdle); err != nil {
		return nil, errors.Wrap(err, "failed to wait for service to enter idle state")
	}
	if err := pw.Expect(wCtx, shillconst.ServicePropertyIsConnected, true); err != nil {
		return nil, errors.Wrap(err, "failed to wait for connection")
	}

	return &wifi.SuspendAssertConnectResponse{ReconnectTime: time.Since(resumeStartTime).Nanoseconds()}, nil
}

// Suspend suspends the DUT.
func (s *ShillService) Suspend(ctx context.Context, req *wifi.SuspendRequest) (*wifi.SuspendResponse, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	interval, err := suspend(ctx, time.Duration(req.WakeUpTimeout), req.CheckEarlyWake)
	if err != nil {

		return nil, err
	}
	return &wifi.SuspendResponse{
		SuspendTime: interval.Nanoseconds(),
	}, nil
}

// GetGlobalFTProperty returns the WiFi.GlobalFTEnabled manager property value.
func (s *ShillService) GetGlobalFTProperty(ctx context.Context, _ *empty.Empty) (*wifi.GetGlobalFTPropertyResponse, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a shill manager")
	}

	props, err := m.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the shill manager properties")
	}
	enabled, err := props.GetBool(shillconst.ManagerPropertyGlobalFTEnabled)
	if err != nil {
		return nil, err
	}
	return &wifi.GetGlobalFTPropertyResponse{Enabled: enabled}, nil
}

// SetGlobalFTProperty set the WiFi.GlobalFTEnabled manager property value.
func (s *ShillService) SetGlobalFTProperty(ctx context.Context, req *wifi.SetGlobalFTPropertyRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a shill manager")
	}
	if err := m.SetProperty(ctx, shillconst.ManagerPropertyGlobalFTEnabled, req.Enabled); err != nil {
		return nil, errors.Wrapf(err, "failed to set the shill manager property %s with value %v", shillconst.ManagerPropertyGlobalFTEnabled, req.Enabled)
	}
	return &empty.Empty{}, nil
}

// FlushBSS flushes BSS entries over the specified age from wpa_supplicant's cache.
func (s *ShillService) FlushBSS(ctx context.Context, req *wifi.FlushBSSRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	supplicant, err := wpasupplicant.NewSupplicant(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to wpa_supplicant")
	}
	iface, err := supplicant.GetInterface(ctx, req.InterfaceName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interface object paths")
	}
	ageThreshold := uint32(time.Duration(req.Age).Seconds())
	if err := iface.FlushBSS(ctx, ageThreshold); err != nil {
		return nil, errors.Wrap(err, "failed to call FlushBSS method")
	}
	return &empty.Empty{}, nil
}

// ResetTest is the main body of the Reset test, which resets/suspends and verifies the connection for several times.
func (s *ShillService) ResetTest(ctx context.Context, req *wifi.ResetTestRequest) (*empty.Empty, error) {
	const (
		resetNum           = 15
		suspendNum         = 5
		suspendDuration    = time.Second * 10
		idleConnectTimeout = time.Second * 20
		pingCount          = 10
		pingInterval       = 1 // In seconds.
		pingLossThreshold  = 20.0

		mwifiexFormat = "/sys/kernel/debug/mwifiex/%s/reset"
		ath10kFormat  = "/sys/kernel/debug/ieee80211/%s/ath10k/simulate_fw_crash"
		// Possible reset paths for Intel wireless NICs are:
		// 1. /sys/kernel/debug/iwlwifi/{iface}/iwlmvm/fw_restart
		//    Logs look like: iwlwifi 0000:00:0c.0: 0x00000038 | BAD_COMMAND
		//    This also triggers a register dump after the restart.
		// 2. /sys/kernel/debug/iwlwifi/{iface}/iwlmvm/fw_nmi
		//    Logs look like: iwlwifi 0000:00:0c.0: 0x00000084 | NMI_INTERRUPT_UNKNOWN
		//    This triggers a "hardware restart" once the NMI is processed
		// 3. /sys/kernel/debug/iwlwifi/{iface}/iwlmvm/fw_dbg_collect
		//    The third one is a mechanism to collect firmware debug dumps, that
		//    effectively causes a restart, but we'll leave it aside for now.
		iwlwifiFormat = "/sys/kernel/debug/iwlwifi/%s/iwlmvm/fw_restart"

		// The path is used to check for wcn399x device.
		ath10kDeviceFormat = "/sys/class/net/%s/device/of_node/compatible"
		wcn3990Signature   = "qcom,wcn3990-wifi"

		mwifiexTimeout  = time.Second * 20
		mwifiexInterval = time.Millisecond * 500
	)

	fileExists := func(file string) bool {
		_, err := os.Stat(file)
		return !os.IsNotExist(err)
	}
	writeStringToFile := func(file, content string) error {
		return ioutil.WriteFile(file, []byte(content), 0444)
	}
	pingOnce := func(ctx context.Context) error {
		pingOps := []ping.Option{
			ping.Count(pingCount),
			ping.Interval(pingInterval),
		}
		res, err := local_ping.NewLocalRunner().Ping(ctx, req.ServerIp, pingOps...)
		if err != nil {
			return errors.Wrap(err, "failed to ping from the DUT")
		}
		testing.ContextLogf(ctx, "ping result: %+v", res)
		if res.Loss > pingLossThreshold {
			return errors.Errorf("unexpected packet loss percentage: got %g%%, want <= %g%%", res.Loss, pingLossThreshold)
		}
		return nil
	}
	// Asserts that after f() is called, shill sees the service changes state to Idle then to IsConnected.
	// The reason to pass f() is because we need to set up a shill property watcher before the f() is called.
	assertIdleAndConnect := func(ctx context.Context, f func(ctx context.Context) error) error {
		service, err := shill.NewService(ctx, dbus.ObjectPath(req.ServicePath))
		if err != nil {
			return errors.Wrap(err, "failed to create a shill service")
		}
		pw, err := service.CreateWatcher(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create shill property watcher")
		}
		defer pw.Close(ctx)
		ctx, cancel := ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		if err := f(ctx); err != nil {
			return err
		}

		if err := pw.Expect(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateIdle); err != nil {
			return errors.Wrap(err, "failed to wait for the service enters Idle state")
		}
		if err := pw.Expect(ctx, shillconst.ServicePropertyIsConnected, true); err != nil {
			return errors.Wrap(err, "failed to wait for the service becomes IsConnected")
		}

		return nil
	}

	// Utils for mwifiex, ath10k, and iwlwifi drivers. *ResetPath return the reset paths in sysfs if they exist. *Reset do the reset once.
	mwifiexResetPath := func(_ context.Context, iface string) (string, error) {
		resetPath := fmt.Sprintf(mwifiexFormat, iface)
		if !fileExists(resetPath) {
			return "", errors.Errorf("mwifiex reset path %q does not exist", resetPath)
		}
		return resetPath, nil
	}
	mwifiexReset := func(ctx context.Context, resetPath string) error {
		ctx, cancel := context.WithTimeout(ctx, mwifiexTimeout)
		defer cancel()

		// We aren't guaranteed to receive a disconnect event, but shill will at least notice the adapter went away.
		if err := assertIdleAndConnect(ctx, func(_ context.Context) error {
			if err := writeStringToFile(resetPath, "1"); err != nil {
				return errors.Wrapf(err, "failed to write to the reset path %q", resetPath)
			}
			return nil
		}); err != nil {
			return err
		}

		return testing.Poll(ctx, func(ctx context.Context) error {
			if !fileExists(resetPath) {
				return errors.Errorf("failed to wait for reset interface file %q to come back", resetPath)
			}
			return nil
		}, &testing.PollOptions{
			// Not setting Timeout here as we have shortened the ctx in the beginning of the function.
			Interval: mwifiexInterval,
		})
	}
	ath10kResetPath := func(ctx context.Context, iface string) (string, error) {
		phy, err := network_iface.NewInterface(iface).PhyName(ctx)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get the phy name of the WiFi interface (%s)", iface)
		}
		resetPath := fmt.Sprintf(ath10kFormat, phy)
		if !fileExists(resetPath) {
			return "", errors.Errorf("ath10k reset path %q does not exist", resetPath)
		}
		return resetPath, nil
	}
	ath10kReset := func(_ context.Context, resetPath string) error {
		// Simulate ath10k firmware crash. mac80211 handles firmware crashes transparently, so we don't expect a full disconnect/reconnet event.
		// From ath10k debugfs:
		//   To simulate firmware crash write one of the keywords to this file:
		//   `soft`       - This will send WMI_FORCE_FW_HANG_ASSERT to firmware if FW supports that command.
		//   `hard`       - This will send to firmware command with illegal parameters causing firmware crash.
		//   `assert`     - This will send special illegal parameter to firmware to cause assert failure and crash.
		//   `hw-restart` - This will simply queue hw restart without fw/hw actually crashing.
		if err := writeStringToFile(resetPath, "soft"); err != nil {
			return errors.Wrapf(err, "failed to write to the reset path %q", resetPath)
		}
		return nil
	}
	ath10kWCN3990ResetPath := func(ctx context.Context, iface string) (string, error) {
		rp, err := ath10kResetPath(ctx, iface)
		if err != nil {
			return "", err
		}
		b, err := ioutil.ReadFile(fmt.Sprintf(ath10kDeviceFormat, iface))
		if err != nil {
			return "", err
		}
		for _, d := range strings.Split(string(b), "\000") {
			if d == wcn3990Signature {
				return rp, nil
			}
		}
		return "", errors.New("not a wcn3990 device")
	}
	ath10kWCN3990Reset := func(ctx context.Context, resetPath string) error {
		return assertIdleAndConnect(ctx, func(ctx context.Context) error {
			return ath10kReset(ctx, resetPath)
		})
	}
	iwlwifiResetPath := func(ctx context.Context, iface string) (string, error) {
		par, err := network_iface.NewInterface(iface).ParentDeviceName(ctx)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get the parent device name of the WiFi interface (%s)", iface)
		}
		resetPath := fmt.Sprintf(iwlwifiFormat, par)
		if !fileExists(resetPath) {
			return "", errors.Errorf("iwlwifi reset path %q does not exist", resetPath)
		}
		return resetPath, nil
	}
	iwlwifiReset := func(_ context.Context, resetPath string) error {
		// Simulate iwlwifi firmware crash. mac80211 handles firmware crashes transparently, so we don't expect a full disconnect/reconnet event.
		if err := writeStringToFile(resetPath, "1"); err != nil {
			return errors.Wrapf(err, "failed to write to the reset path %q", resetPath)
		}
		return nil
	}

	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager proxy")
	}
	iface, err := shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi interface")
	}

	// Find the first workable reset path and function by checking the presence of a reset path from the three WiFi module families.
	var reset func(context.Context, string) error
	var resetPath string
	var resetPathErrs []string
	for _, v := range []struct {
		reset     func(context.Context, string) error
		resetPath func(context.Context, string) (string, error)
	}{
		{mwifiexReset, mwifiexResetPath},
		// WCN3990 belongs to ath10k Wi-Fi family. Evaluate the specific Wi-Fi module detectors first.
		{ath10kWCN3990Reset, ath10kWCN3990ResetPath},
		{ath10kReset, ath10kResetPath},
		{iwlwifiReset, iwlwifiResetPath},
	} {
		rp, err := v.resetPath(ctx, iface)
		if err == nil {
			reset = v.reset
			resetPath = rp
			break
		}
		// Collect the errors from *ResetPath functions in case we can't find any available reset path.
		resetPathErrs = append(resetPathErrs, err.Error())
	}
	if reset == nil {
		return nil, errors.Errorf("no valid reset path, err=%s", strings.Join(resetPathErrs, ", err="))
	}

	testing.ContextLogf(ctx, "Reset WiFi device for %d times then perform suspend/resume test; totally %d rounds; reset path=%s", resetNum, suspendNum, resetPath)

	for i := 0; i < suspendNum; i++ {
		testing.ContextLogf(ctx, "Start reseting for %d times", resetNum)
		for j := 0; j < resetNum; j++ {
			testing.ContextLogf(ctx, "Reset %d", j+1)
			if err := reset(ctx, resetPath); err != nil {
				return nil, errors.Wrap(err, "failed to reset the WiFi interface")
			}
			if err := pingOnce(ctx); err != nil {
				return nil, errors.Wrap(err, "failed to verify connection after reset")
			}
		}
		testing.ContextLogf(ctx, "Finished %d resetings; Start suspending for %s", resetNum, suspendDuration)
		// Suspend for suspendDuration; resume; then wait for the service enters Idle state and IsConnected in order.
		if err := func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, suspendDuration+idleConnectTimeout)
			defer cancel()

			return assertIdleAndConnect(ctx, func(ctx context.Context) error {
				if _, err := suspend(ctx, suspendDuration, true /* checkEarlyWake */); err != nil {
					return errors.Wrap(err, "failed to suspend")
				}
				return nil
			})
		}(ctx); err != nil {
			return nil, err
		}
		if err := pingOnce(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to verify connection after suspend")
		}
	}
	return &empty.Empty{}, nil
}

// HealthCheck checks if the DUT has a WiFi device. If not, we may need to reboot the DUT.
func (s *ShillService) HealthCheck(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager")
	}
	_, err = shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "could not get a WiFi interface")
	}

	return &empty.Empty{}, nil
}

// GetLoggingConfig returns the logging configuration the device currently uses.
func (s *ShillService) GetLoggingConfig(ctx context.Context, e *empty.Empty) (*wifi.GetLoggingConfigResponse, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager proxy")
	}

	level, err := manager.GetDebugLevel(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the debug level")
	}

	tags, err := manager.GetDebugTags(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the debug tags")
	}

	return &wifi.GetLoggingConfigResponse{
		DebugLevel: int32(level),
		DebugTags:  tags,
	}, nil
}

// SetLoggingConfig sets the device logging configuration.
func (s *ShillService) SetLoggingConfig(ctx context.Context, req *wifi.SetLoggingConfigRequest) (*empty.Empty, error) {
	ctx, cancel := reserveForReturn(ctx)
	defer cancel()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shill manager proxy")
	}

	if err := manager.SetDebugLevel(ctx, int(req.DebugLevel)); err != nil {
		return nil, errors.Wrap(err, "failed to set the debug level")
	}

	if err := manager.SetDebugTags(ctx, req.DebugTags); err != nil {
		return nil, errors.Wrap(err, "failed to set the debug tags")
	}

	return &empty.Empty{}, nil
}

// GetWakeOnWifi returns the wake on WiFi related properties of WiFi device.
func (s *ShillService) GetWakeOnWifi(ctx context.Context, _ *empty.Empty) (*wifi.GetWakeOnWifiResponse, error) {
	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}
	props, err := dev.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi device properties")
	}
	allowed, err := props.GetBool(shillconst.DevicePropertyWakeOnWiFiAllowed)
	if err != nil {
		return nil, err
	}
	features, err := props.GetString(shillconst.DevicePropertyWakeOnWiFiFeaturesEnabled)
	if err != nil {
		return nil, err
	}
	netDetectScanPeriod, err := props.GetUint32(shillconst.DevicePropertyNetDetectScanPeriodSeconds)
	if err != nil {
		return nil, err
	}

	return &wifi.GetWakeOnWifiResponse{
		Config: &wifi.WakeOnWifiConfig{
			Allowed:             allowed,
			Features:            features,
			NetDetectScanPeriod: netDetectScanPeriod,
		},
	}, nil
}

// SetWakeOnWifi sets wake on WiFi related property of WiFi device.
func (s *ShillService) SetWakeOnWifi(ctx context.Context, req *wifi.SetWakeOnWifiRequest) (*empty.Empty, error) {
	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}

	config := req.Config

	// Currently, we block WoWiFi enablement behind the "allowed" flag.
	// Check if the setting is valid before further action.
	if !config.Allowed && config.Features != shillconst.WakeOnWiFiFeaturesNone {
		return nil, errors.Errorf("WoWiFi not allowed but expected features=%q to be enabled", config.Features)
	}

	// Set allowed first.
	if err := dev.SetProperty(ctx, shillconst.DevicePropertyWakeOnWiFiAllowed, config.Allowed); err != nil {
		return nil, errors.Wrapf(err, "failed to set WakeOnWiFiAllowed to %t", config.Allowed)
	}
	// Only set features when allowed as it should be always "none" when not allowed.
	if config.Allowed {
		if err := dev.SetProperty(ctx, shillconst.DevicePropertyWakeOnWiFiFeaturesEnabled, config.Features); err != nil {
			return nil, errors.Wrapf(err, "failed to set the WakeOnWiFiFeaturesEnabled property to %s", config.Features)
		}
	}
	if err := dev.SetProperty(ctx, shillconst.DevicePropertyNetDetectScanPeriodSeconds, config.NetDetectScanPeriod); err != nil {
		return nil, errors.Wrapf(err, "failed to set NetDetectScanPeriod to %d seconds", config.NetDetectScanPeriod)
	}

	return &empty.Empty{}, nil
}

// CheckLastWakeReason checks if the last wake reason of WiFi device is as expected.
func (s *ShillService) CheckLastWakeReason(ctx context.Context, req *wifi.CheckLastWakeReasonRequest) (*empty.Empty, error) {
	_, dev, err := s.wifiDev(ctx)
	if err != nil {
		return nil, err
	}

	props, err := dev.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi device properties")
	}
	reason, err := props.GetString(shillconst.DevicePropertyLastWakeReason)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get LastWakeReason property")
	}
	// TODO(b/187362093): The check can race with NL80211 message to shill. Could improve
	// robustness by waiting for PropertyChanged.
	if reason != req.Reason {
		return nil, errors.Wrapf(err, "unexpected LastWakeReason, got %s, want %s", reason, req.Reason)
	}
	return &empty.Empty{}, nil
}
