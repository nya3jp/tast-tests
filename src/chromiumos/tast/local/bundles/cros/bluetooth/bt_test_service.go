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
	s            *testing.ServiceState
	cr           *chrome.Chrome
	bluezAdapter *bluez.Adapter
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
