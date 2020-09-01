// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	//"chromiumos/tast/local/chrome/ui"
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
	cr, err := chrome.New(
		ctx,
		chrome.KeepState(),
		chrome.NoLogin(),
		chrome.DisableNoStartupWindow(),
		chrome.LoadSigninProfileExtension(req.Credentials),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	//tLoginConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create login test API connection")
	}
	defer tLoginConn.Close()

	const pauseDuration = time.Second
	testing.Sleep(ctx, pauseDuration*10)

	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.bluetoothPrivate.setAdapterState(
		      {powered: %s}, function() {
		    resolve(chrome.runtime.lastError ? chrome.runtime.lastError.message : "");
		  });
		})`, strconv.FormatBool(req.State))

	msg := ""
	if err = tLoginConn.EvalPromise(ctx, expr, &msg); err != nil {
		return nil, errors.Wrap(err, "failed to get display info")
	} else if msg != "" {
		return nil, errors.New(msg)
	}
	return &empty.Empty{}, err
}

func (s *BluetoothService) GetBluetoothStatus(ctx context.Context, _ *empty.Empty) (*network.GetBluetoothStatusResponse, error) {
	status, err := bluetoothStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error in querying bluetooth status")
	}
	return &network.GetBluetoothStatusResponse{Status: status}, nil
}
