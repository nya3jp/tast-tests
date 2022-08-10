// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/chrome"
	pb "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterBTTestServiceServer(srv, &BTTestService{s: s})
		},
	})
}

// BTTestService implements tast.cros.bluetooth.BTTestService.
type BTTestService struct {
	s                *testing.ServiceState
	cr               *chrome.Chrome
	bluezAdapter     *bluez.Adapter
	connectedDevices map[string]*bluez.Device
}

// ChromeNew logs into chrome. ChromeClose must be called later.
func (bts *BTTestService) ChromeNew(ctx context.Context, request *pb.ChromeNewRequest) (*emptypb.Empty, error) {
	if bts.cr != nil {
		return nil, errors.New("chrome already available")
	}
	var chromeOpts []chrome.Option

	if len(request.EnableFeatures) > 0 {
		chromeOpts = append(chromeOpts, chrome.EnableFeatures(request.EnableFeatures...))
	}

	if len(request.DisableFeatures) > 0 {
		chromeOpts = append(chromeOpts, chrome.DisableFeatures(request.DisableFeatures...))
	}

	if request.NoLogin {
		chromeOpts = append(chromeOpts, chrome.NoLogin())
	}
	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new Chrome")
	}
	bts.cr = cr
	return &emptypb.Empty{}, nil
}

// ChromeClose cleans up resources from ChromeNew.
func (bts *BTTestService) ChromeClose(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if bts.cr == nil {
		return nil, errors.New("no chrome to close, call ChromeNew first")
	}
	if err := bts.cr.Close(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to close chrome")
	}
	bts.cr = nil
	return &emptypb.Empty{}, nil
}

// EnableBluetoothAdapter powers on the bluetooth adapter and waits for it to
// be enabled.
func (bts *BTTestService) EnableBluetoothAdapter(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if err := bluez.Enable(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to enable bluetooth adapter")
	}
	if err := bluez.PollForBTEnabled(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for bluetooth adapter to be enabled")
	}
	if adapters, err := bluez.Adapters(ctx); err == nil {
		bts.bluezAdapter = adapters[0]
	} else {
		return nil, errors.Wrap(err, "failed to get bluetooth adapters")
	}
	return &emptypb.Empty{}, nil
}

// DisableBluetoothAdapter powers off the bluetooth adapter.
func (bts *BTTestService) DisableBluetoothAdapter(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if err := bluez.Disable(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disable bluetooth adapter")
	}
	if err := bluez.PollForBTDisabled(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for bluetooth adapter to be disabled")
	}
	bts.bluezAdapter = nil
	return &emptypb.Empty{}, nil
}

// DisconnectAllDevices disconnects all connected bluetooth devices.
func (bts *BTTestService) DisconnectAllDevices(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if err := bluez.DisconnectAllDevices(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disconnect all bluetooth devices")
	}
	return &emptypb.Empty{}, nil
}

// DiscoverDevice confirms that the DUT can discover the provided bluetooth
// device. Fails if the device is not found or if the discovered matching
// device's attributes do not match those provided.
func (bts *BTTestService) DiscoverDevice(ctx context.Context, request *pb.DiscoverDeviceRequest) (*emptypb.Empty, error) {
	if request.Device == nil || request.Device.AdvertisedName == "" ||
		request.Device.MacAddress == "" {
		return nil, errors.New("incomplete DiscoverDevice request")
	}
	if _, err := bts.discoverDeviceByAddress(ctx, request.Device.MacAddress, request.Device.AdvertisedName); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (bts *BTTestService) discoverDeviceByAddress(ctx context.Context, targetDeviceAddress, expectedDeviceName string) (*bluez.Device, error) {
	if bts.bluezAdapter == nil {
		return nil, errors.New("bluetooth adapter not initialized, call EnableBluetoothAdapter first")
	}
	if err := bts.bluezAdapter.StartDiscovery(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start discovery")
	}
	var btDevice *bluez.Device
	testing.ContextLogf(ctx, "Polling for discovery of device with address %q", targetDeviceAddress)
	if pollErr := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		btDevice, err = bluez.DeviceByAddress(ctx, targetDeviceAddress)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 250 * time.Millisecond}); pollErr != nil {
		baseErr := errors.Wrapf(pollErr, "timeout waiting for discover device with address %q", targetDeviceAddress)
		// Failed to find the specific device. Attempt to include a list of devices that were found in the error message.
		devices, err := bts.discoverDevices(ctx)
		if err != nil {
			return nil, baseErr
		}
		devicesStr := make([]string, len(devices))
		for i, device := range devices {
			devicesStr[i] = device.String()
		}
		sort.Strings(devicesStr)
		return nil, errors.Wrapf(
			pollErr,
			"timeout waiting for discover device with address %q. Found %d other devices (%v)",
			targetDeviceAddress,
			len(devices),
			strings.Join(devicesStr, ", "))
	}
	testing.ContextLogf(ctx, "Discovered device with address %q at dbus path %q", targetDeviceAddress, btDevice.Path())
	if err := bts.bluezAdapter.StopDiscovery(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start discovery")
	}

	// Validate discovered device is intended device.
	btDeviceAddr, err := btDevice.Address(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get address of connected device")
	}
	if btDeviceAddr != targetDeviceAddress {
		return nil, errors.Errorf("discovered device with address %q does not match expected address %q", btDeviceAddr, targetDeviceAddress)
	}
	btDeviceName, err := btDevice.Name(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get name of connected device")
	}
	if btDeviceName != expectedDeviceName {
		return nil, errors.Errorf("discovered device with name %q does not match expected name %q", btDeviceName, expectedDeviceName)
	}
	return btDevice, nil
}

func (bts *BTTestService) discoverDevices(ctx context.Context) ([]*pb.Device, error) {
	foundDevices, err := bluez.Devices(ctx)
	if err != nil {
		return nil, err
	}
	var devices = make([]*pb.Device, len(foundDevices))
	for i, foundDevice := range foundDevices {
		name, err := foundDevice.Name(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get name of found device")
		}
		macAddress, err := foundDevice.Address(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get mac address of found device")
		}
		devices[i] = &pb.Device{
			AdvertisedName: name,
			MacAddress:     macAddress,
		}
	}
	return devices, nil
}

// PairAndConnectDevice pairs and connects to the specified Device.
func (bts *BTTestService) PairAndConnectDevice(ctx context.Context, request *pb.PairAndConnectDeviceRequest) (*emptypb.Empty, error) {
	if request.Device == nil || request.Device.AdvertisedName == "" ||
		request.Device.MacAddress == "" {
		return nil, errors.New("incomplete PairAndConnectDevice request")
	}

	// Attempt to discover device.
	btDevice, err := bts.discoverDeviceByAddress(ctx, request.Device.MacAddress, request.Device.AdvertisedName)
	if err != nil {
		return nil, err
	}

	// Pair BT Device.
	isPaired, err := btDevice.Paired(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if device is paired")
	}
	if isPaired {
		if request.ForceNewPair {
			testing.ContextLogf(ctx, "Removing and rediscovering already paired device at dbus path %q", btDevice.Path())
			if err := bts.bluezAdapter.RemoveDevice(ctx, btDevice.Path()); err != nil {
				return nil, errors.Wrapf(err, "failed to remove paired device at dbus path %q", btDevice.Path())
			}
			btDevice, err = bts.discoverDeviceByAddress(ctx, request.Device.MacAddress, request.Device.AdvertisedName)
			if err != nil {
				return nil, errors.Wrap(err, "failed rediscover device after removal")
			}
			isPaired = false
		} else {
			testing.ContextLogf(ctx, "Skipping pairing step as device at dbus path %q is already paired", btDevice.Path())
		}
	}
	if !isPaired {
		if request.Device.HasPinCode {
			// Prepare to handle pin authorization.
			testing.ContextLogf(ctx, "Preparing to authorize paring with pin code %q", request.Device.PinCode)
			testing.ContextLog(ctx, "Retrieving authentication AgentManager")
			agentManagers, err := bluez.AgentManagers(ctx)
			if err != nil || len(agentManagers) == 0 {
				return nil, errors.Wrap(err, "failed to get AgentManager")
			}
			agentManager := agentManagers[0]
			testing.ContextLogf(ctx, "Using authentication AgentManager at dbus path %q", agentManager.DBusObject().ObjectPath())
			agents, err := bluez.Agents(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get existing Agents")
			}
			testing.ContextLogf(ctx, "Found %d Agents", len(agents))

			testing.ContextLog(ctx, "Creating new authentication Agent")
			agent, err := bluez.NewAgent(ctx, "")
			if err != nil {
				return nil, errors.Wrap(err, "failed to create new authentication Agent")
			}
			testing.ContextLogf(ctx, "Using authentication Agent at dbus path %q", agent.DBusObject().ObjectPath())
			if err := agent.ExportAgentDelegate(bluez.NewSimplePinAgentDelegate(ctx, request.Device.PinCode)); err != nil {
				return nil, errors.Wrap(err, "failed to export AgentDelegate")
			}
			if err := agentManager.RegisterAgent(ctx, agent.DBusObject().ObjectPath(), "KeyboardDisplay"); err != nil {
				return nil, errors.Wrapf(err, "failed to register Agent %q with AgentManager %q", agent.DBusObject().ObjectPath(), agentManager.DBusObject().ObjectPath())
			}
			if err := agentManager.RequestDefaultAgent(ctx, agent.DBusObject().ObjectPath()); err != nil {
				return nil, errors.Wrapf(err, "failed to register Agent %q with AgentManager %q as default", agent.DBusObject().ObjectPath(), agentManager.DBusObject().ObjectPath())
			}

			agents, err = bluez.Agents(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get existing Agents")
			}
			testing.ContextLogf(ctx, "Found %d Agents", len(agents))

			// Attempt paring.
			testing.ContextLogf(ctx, "Pairing device at dbus path %q", btDevice.Path())
			if err := btDevice.Pair(ctx); err != nil {
				return nil, errors.Wrap(err, "failed to pair bluetooth device")
			}

			// Cleanup pin authentication handling.
			if err := agent.ClearExportedAgentDelegate(); err != nil {
				return nil, errors.Wrapf(err, "failed to clear exported AgentDelegate for Agent at %q", agent.DBusObject().ObjectPath())
			}
			if err := agentManager.UnregisterAgent(ctx, agent.DBusObject().ObjectPath()); err != nil {
				return nil, errors.Wrapf(err, "failed to unregister Agent %q with AgentManager %q", agent.DBusObject().ObjectPath(), agentManager.DBusObject().ObjectPath())
			}
		} else {
			testing.ContextLogf(ctx, "Pairing device at dbus path %q", btDevice.Path())
			if err := btDevice.Pair(ctx); err != nil {
				return nil, errors.Wrap(err, "failed to pair bluetooth device")
			}
		}
	}

	// Get connected status of BT device and connect if not already connected.
	testing.ContextLogf(ctx, "Connecting to device at dbus path %q", btDevice.Path())
	isConnected, err := btDevice.Connected(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device connected status")
	}
	if isConnected {
		if request.ForceNewConnect {
			testing.ContextLogf(ctx, "Disconnecting already connected device at dbus path %q", btDevice.Path())
			if err := btDevice.Disconnect(ctx); err != nil {
				return nil, errors.Wrap(err, "failed to disconnect bluetooth device")
			}
			isConnected = false
		} else {
			testing.ContextLogf(ctx, "Skipping connect step as device at dbus path %q is already connected", btDevice.Path())
		}
	}
	if !isConnected {
		testing.ContextLogf(ctx, "Connecting to device at dbus path %q", btDevice.Path())
		if err := btDevice.Connect(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to connect bluetooth device")
		}
	}

	// Store device for later requests.
	bts.connectedDevices[request.Device.MacAddress] = btDevice

	return &emptypb.Empty{}, nil
}
