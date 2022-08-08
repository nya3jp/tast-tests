// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"sort"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
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
	bluezAdapter     *bluetooth.Adapter
	connectedDevices map[string]*bluetooth.Device
}

// ChromeNew logs into chrome. ChromeClose must be called later.
func (bts *BTTestService) ChromeNew(ctx context.Context, request *pb.ChromeNewRequest) (*emptypb.Empty, error) {
	if bts.cr != nil {
		return nil, errors.New("chrome already available")
	}
	var chromeOpts []chrome.Option
	if request.BluetoothRevampEnabled {
		chromeOpts = []chrome.Option{chrome.EnableFeatures("BluetoothRevamp")}
	} else {
		chromeOpts = []chrome.Option{chrome.DisableFeatures("BluetoothRevamp")}
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
	return &emptypb.Empty{}, nil
}

// EnableBluetoothAdapter powers on the bluetooth adapter and waits for it to
// be enabled.
func (bts *BTTestService) EnableBluetoothAdapter(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if err := bluetooth.Enable(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to enable bluetooth adapter")
	}
	if err := bluetooth.PollForBTEnabled(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for bluetooth adapter to be enabled")
	}
	if adapters, err := bluetooth.Adapters(ctx); err == nil {
		bts.bluezAdapter = adapters[0]
	} else {
		return nil, errors.Wrap(err, "failed to get bluetooth adapters")
	}
	return &emptypb.Empty{}, nil
}

// DisableBluetoothAdapter powers off the bluetooth adapter.
func (bts *BTTestService) DisableBluetoothAdapter(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if err := bluetooth.Disable(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disable bluetooth adapter")
	}
	if err := bluetooth.PollForBTDisabled(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for bluetooth adapter to be disabled")
	}
	bts.bluezAdapter = nil
	return &emptypb.Empty{}, nil
}

// PairAndConnectDevice pairs and connects to the specified Device.
func (bts *BTTestService) PairAndConnectDevice(ctx context.Context, request *pb.PairAndConnectDeviceRequest) (*emptypb.Empty, error) {
	if bts.bluezAdapter == nil {
		return nil, errors.New("bluetooth adapter not initialized, call EnableBluetoothAdapter first")
	}
	if request.Device == nil || request.Device.Alias == "" || request.Device.MacAddress == "" {
		return nil, errors.New("device alias and mac address required")
	}

	adapter := bts.bluezAdapter

	if err := adapter.StartDiscovery(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start discovery")
	}

	// Wait for a specific BT device to be found.
	var btDevice *bluetooth.Device
	if pollErr := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		btDevice, err = bluetooth.DeviceByAlias(ctx, request.Device.Alias)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 250 * time.Millisecond}); pollErr != nil {
		baseErr := errors.Wrapf(pollErr, "timeout waiting for discover device with alias %q", request.Device.Alias)
		// Failed to find the specific device. Attempt to include a list of devices that were found in the error message.
		foundDevices, err := bluetooth.Devices(ctx)
		if err != nil {
			return nil, baseErr
		}
		foundDeviceAliases := make([]string, len(foundDevices))
		for i := 0; i < len(foundDevices); i++ {
			alias, err := foundDevices[i].Alias(ctx)
			if err != nil {
				return nil, baseErr
			}
			foundDeviceAliases[i] = alias
		}
		sort.Strings(foundDeviceAliases)
		return nil, errors.Wrapf(pollErr, "timeout waiting for discover device with alias %q but did find %d other devices (%v)", request.Device.Alias, len(foundDevices), foundDeviceAliases)
	}

	// Pair BT Device.
	isPaired, err := btDevice.Paired(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if device is paired")
	}
	if !isPaired {
		if err := btDevice.Pair(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to pair bluetooth device")
		}
	}

	// Get connected status of BT device and connect if not already connected.
	isConnected, err := btDevice.Connected(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device connected status")
	}
	if !isConnected {
		if err := btDevice.Connect(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to connect bluetooth device")
		}
	}

	// Validate connected device is intended device.
	btDeviceAddr, err := btDevice.Address(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get address of connected device")
	}
	if btDeviceAddr != request.Device.MacAddress {
		return nil, errors.Errorf("connected device address %q does not match expected address %q", btDeviceAddr, request.Device.MacAddress)
	}
	btDeviceAlias, err := btDevice.Alias(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get alias of connected device")
	}
	if btDeviceAlias != request.Device.Alias {
		return nil, errors.Errorf("connected device alias %q does not match expected alias %q", btDeviceAlias, request.Device.Alias)
	}

	// Store device for later requests.
	bts.connectedDevices[btDeviceAddr] = btDevice

	return &emptypb.Empty{}, nil
}

// DisconnectAllDevices disconnects all connected bluetooth devices.
func (bts *BTTestService) DisconnectAllDevices(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if err := bluetooth.DisconnectAllDevices(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disconnect all bluetooth devices")
	}
	return &emptypb.Empty{}, nil
}
