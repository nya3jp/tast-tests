// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterBluetoothServiceServer(srv, &BluetoothService{s: s})
		},
	})
}

// BluetoothService implements tast.cros.network.BluetoothService gRPC service.
type BluetoothService struct {
	s *testing.ServiceState
}

// SetBluetoothPowered sets the Bluetooth adapter power status via settingsPrivate. This setting persists across reboots.
func (s *BluetoothService) SetBluetoothPowered(ctx context.Context, req *network.SetBluetoothPoweredRequest) (*empty.Empty, error) {
	cr, err := chrome.New(
		ctx,
		chrome.KeepState(),
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(req.Credentials),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create login test API connection")
	}

	// Toggling this settingsPrivate setting should be sufficient to toggle Bluetooth as well as its boot preference.
	if err := tLoginConn.Call(ctx, nil, `tast.promisify(chrome.settingsPrivate.setPref)`, "ash.system.bluetooth.adapter_enabled", req.Powered); err != nil {
		return nil, err
	}
	var enabled struct {
		Value bool `json:"value"`
	}
	if err := tLoginConn.Call(ctx, &enabled, "tast.promisify(chrome.settingsPrivate.getPref)", "ash.system.bluetooth.adapter_enabled"); err != nil {
		return nil, err
	}

	if enabled.Value != req.Powered {
		return nil, errors.Errorf("bad VALUE, wanted %s, got %s", strconv.FormatBool(req.Powered), strconv.FormatBool(enabled.Value))
	}

	// Verify that the boot setting is set properly in the Local State.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		enabledVal, err := localstate.UnmarshalPref(browser.TypeAsh, "ash.system.bluetooth.adapter_enabled")
		if err != nil {
			return errors.Wrap(err, "failed to extract bluetooth status from Local State")
		}
		enabled, ok := enabledVal.(bool)
		if !ok || (enabled != req.Powered) {
			return errors.Errorf("ash Bluetooth preference not updated properly: wanted %v, got %v", req.Powered, enabledVal)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  40 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return nil, err
	}

	// Poll until the adapter state has been changed to the correct value.
	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return nil, errors.Errorf("got %d adapters, expected 1 adapter", len(adapters))
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := adapters[0].SetPowered(ctx, req.Powered); err != nil {
			return errors.Wrapf(err, "could not set Bluetooth power state to %t", req.Powered)
		}
		if res, err := adapters[0].Powered(ctx); err != nil {
			return errors.Wrap(err, "could not get Bluetooth power state")
		} else if res != req.Powered {
			return errors.Errorf("Bluetooth adapter state not changed to %s after toggle", strconv.FormatBool(req.Powered))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 100 * time.Millisecond,
	}); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// GetBluetoothBootPref gets the Bluetooth boot preference.
func (s *BluetoothService) GetBluetoothBootPref(ctx context.Context, req *network.GetBluetoothBootPrefRequest) (*network.GetBluetoothBootPrefResponse, error) {
	ctx, st := timing.Start(ctx, "GetBluetoothBootPref")
	defer st.End()
	cr, err := chrome.New(
		ctx,
		chrome.KeepState(),
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(req.Credentials),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create login test API connection")
	}

	// Get Bluetooth pref.
	var enabled struct {
		Value bool `json:"value"`
	}
	if err := tLoginConn.Call(ctx, &enabled, "tast.promisify(chrome.settingsPrivate.getPref)", "ash.system.bluetooth.adapter_enabled"); err != nil {
		return nil, err
	}
	return &network.GetBluetoothBootPrefResponse{Persistent: enabled.Value}, nil
}

// SetBluetoothPoweredFast sets the Bluetooth adapter power status via D-Bus. This setting does not persist across boots.
func (s *BluetoothService) SetBluetoothPoweredFast(ctx context.Context, req *network.SetBluetoothPoweredFastRequest) (*empty.Empty, error) {
	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return nil, errors.Errorf("got %d adapters, expected 1 adapter", len(adapters))
	}
	if err := adapters[0].SetPowered(ctx, req.Powered); err != nil {
		return nil, errors.Wrap(err, "could not set Bluetooth power state")
	}

	// Poll until the adapter state has been changed to the correct value.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := adapters[0].Powered(ctx); err != nil {
			return errors.Wrap(err, "could not get Bluetooth power state")
		} else if res != req.Powered {
			return errors.Errorf("Bluetooth adapter state not changed to %s after toggle", strconv.FormatBool(req.Powered))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// GetBluetoothPoweredFast checks whether the Bluetooth adapter is enabled.
func (s *BluetoothService) GetBluetoothPoweredFast(ctx context.Context, _ *empty.Empty) (*network.GetBluetoothPoweredFastResponse, error) {
	ctx, st := timing.Start(ctx, "GetBluetoothPoweredFast")
	defer st.End()
	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return &network.GetBluetoothPoweredFastResponse{Powered: false}, nil
	}
	res, err := adapters[0].Powered(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Bluetooth power state")
	}
	return &network.GetBluetoothPoweredFastResponse{Powered: res}, nil
}

// ValidateBluetoothFunctional checks to see whether the Bluetooth device is usable.
func (s *BluetoothService) ValidateBluetoothFunctional(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return nil, errors.Errorf("got %d adapters, expected 1 adapter", len(adapters))
	}
	// If the Bluetooth device is not usable, the discovery will error out.
	err = adapters[0].StartDiscovery(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Bluetooth adapter discovery")
	}
	// We don't actually care about the discovery contents, just whether or not
	// the discovery failed or not. We can stop the scan immediately.
	err = adapters[0].StopDiscovery(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to stop Bluetooth adapter discovery")
	}
	return &empty.Empty{}, nil
}
