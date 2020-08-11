// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
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

func toggleBluetooth(ctx context.Context, credentials string) error {
	cr, err := chrome.New(
		ctx,
		chrome.KeepState(),
		chrome.NoLogin(),
		chrome.DisableNoStartupWindow(),
		chrome.LoadSigninProfileExtension(credentials),
	)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create login test API connection")
	}
	defer tLoginConn.Close()

	// Find and click the StatusArea via UI. Clicking it opens the Ubertray.
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	statusArea, err := ui.FindWithTimeout(ctx, tLoginConn, params, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the status area (time, battery, etc.)")
	}
	defer statusArea.Release(ctx)
	if err := statusArea.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click status area")
	}

	// Confirm that the system tray is open by checking for the bluetooth button.
	params = ui.FindParams{ClassName: "FeaturePodIconButton"}
	if err := ui.WaitUntilExists(ctx, tLoginConn, params, 10*time.Second); err != nil {
		return errors.Wrap(err, "could not find tray buttons after click")
	}
	elems, err := ui.FindAll(ctx, tLoginConn, params)
	if err != nil {
		return errors.Wrap(err, "tray buttons could not be found")
	}
	var bluetoothButton *ui.Node
	nameMatch := regexp.MustCompile(`^Toggle Bluetooth\. Bluetooth is (on|off)+$`)
	// Find bluetooth button from tray buttons.
	for _, elem := range elems {
		if nameMatch.Match([]byte(elem.Name)) {
			bluetoothButton = elem
			break
		}
	}
	if bluetoothButton == nil {
		return errors.New("could not find bluetooth button")
	}
	defer bluetoothButton.Release(ctx)

	// Toggle the bluetooth button.
	if err := bluetoothButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click bluetooth button")
	}
	return nil
}

func bluetoothStatus(ctx context.Context) (bool, error) {
	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to get Bluetooth adapters")
	}

	if len(adapters) != 1 {
		return false, nil
	}
	adapter := adapters[0]
	return adapter.Powered(ctx)
}

func (s *BluetoothService) SetBluetoothStatus(ctx context.Context, req *network.SetBluetoothStatusRequest) (*empty.Empty, error) {
	state := req.State
	if status, err := bluetoothStatus(ctx); err != nil {
		return nil, errors.Wrap(err, "error in querying bluetooth status")
	} else if status != state {
		if err := toggleBluetooth(ctx, req.Credentials); err != nil {
			return nil, errors.Wrap(err, "failed to change bluetooth status")
		}
		return &empty.Empty{}, nil
	}
	return &empty.Empty{}, nil
}

func (s *BluetoothService) GetBluetoothStatus(ctx context.Context, _ *empty.Empty) (*network.GetBluetoothStatusResponse, error) {
	status, err := bluetoothStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error in querying bluetooth status")
	}
	return &network.GetBluetoothStatusResponse{Status: status}, nil
}
