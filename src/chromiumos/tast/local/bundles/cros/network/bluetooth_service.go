// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
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

// SetBluetoothPowered sets the Bluetooth adapter power status. This setting persists across reboots.
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
	defer tLoginConn.Close()

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
		command := "jq"
		args := []string{
			".ash.system.bluetooth.adapter_enabled",
			"/home/chronos/Local State",
		}
		output, err := testexec.CommandContext(ctx, command, args...).Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		} else if strings.TrimSpace(string(output)) != strconv.FormatBool(req.Powered) {
			return errors.Errorf("ash Bluetooth preference not updated properly: wanted %s, got %s", strconv.FormatBool(req.Powered), string(output))
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  40 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// GetBluetoothPowered checks whether the Bluetooth adapter is enabled as well as the Bluetooth boot preference.
func (s *BluetoothService) GetBluetoothPowered(ctx context.Context, req *network.GetBluetoothPoweredRequest) (*network.GetBluetoothPoweredResponse, error) {
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
	defer tLoginConn.Close()

	// Get Bluetooth pref.
	var enabled struct {
		Value bool `json:"value"`
	}
	if err := tLoginConn.Call(ctx, &enabled, "tast.promisify(chrome.settingsPrivate.getPref)", "ash.system.bluetooth.adapter_enabled"); err != nil {
		return nil, err
	}

	// Get Bluetooth status.
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return &network.GetBluetoothPoweredResponse{Powered: false, Persistent: enabled.Value}, nil
	}
	res, err := adapters[0].Powered(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Bluetooth power state")
	}
	return &network.GetBluetoothPoweredResponse{Powered: res, Persistent: enabled.Value}, nil
}

// ValidateBluetoothFunctional checks to see whether the Bluetooth device is usable.
func (s *BluetoothService) ValidateBluetoothFunctional(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	adapters, err := bluetooth.Adapters(ctx)
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
