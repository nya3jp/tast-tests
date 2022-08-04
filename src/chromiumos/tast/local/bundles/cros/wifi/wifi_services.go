// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/services/cros/wifiservice"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			wifiservice.RegisterWifiServiceServer(srv, &WifiService{s: s})
		},
	})
}

// WifiService implements tast.cros.wifi.Wifi gRPC service.
type WifiService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

// New logins to chrome.
func (c *WifiService) New(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	c.cr = cr
	return &empty.Empty{}, nil
}

// Close chrome login.
func (c *WifiService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := c.cr.Close(ctx)
	c.cr = nil
	return &empty.Empty{}, err
}

// ConnectWifi connects to wifi with given ssid and password.
func (c *WifiService) ConnectWifi(ctx context.Context, req *wifiservice.WifiRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Connecting to wifi")
	// Check and prepare wifi.
	var wifi *shill.WifiManager
	var err error

	if wifi, err = shill.NewWifiManager(ctx, nil); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create shill Wi-Fi manager")
	}
	// Ensure wi-fi is enabled.
	if err := wifi.Enable(ctx, true); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to enable Wi-Fi")
	}
	testing.ContextLog(ctx, "Wi-Fi is enabled")
	if err := wifi.ConnectAP(ctx, req.Ssid, req.Password); err != nil {
		return &empty.Empty{}, errors.Wrapf(err, "failed to connect Wi-Fi AP %s", req.Ssid)
	}
	testing.ContextLogf(ctx, "Wi-Fi AP %s is connected", req.Ssid)
	return &empty.Empty{}, nil
}

// DownEth makes the Ethernet down.
func (c *WifiService) DownEth(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if err := testexec.CommandContext(ctx, "bash", "-c", "ifconfig eth0 down").Run(); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to turn off ethernet")
	}
	return &empty.Empty{}, nil
}

// UpEth makes the Ethernet up.
func (c *WifiService) UpEth(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if err := testexec.CommandContext(ctx, "bash", "-c", "ifconfig eth0 up").Run(); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to turn on ethernet")
	}
	return &empty.Empty{}, nil
}
