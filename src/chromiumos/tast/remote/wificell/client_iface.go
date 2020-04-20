// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/network"
)

// ClientIface provides methods to access properties of test client (DUT)'s network interface.
type ClientIface struct {
	name       string             // Network interface name.
	wifiClient network.WifiClient // Gate to gRPC connection to the DUT.
}

// NewClientInterface creates a ClientIface.
func (tf *TestFixture) NewClientInterface(ctx context.Context) (*ClientIface, error) {
	netIf, err := tf.wifiClient.Interface(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi interface name")
	}
	return &ClientIface{name: netIf.Iface, wifiClient: tf.wifiClient}, nil
}

// IPForInterface returns the IP address for the network interface.
func (c *ClientIface) IPForInterface(ctx context.Context) (string, error) {
	iface := &network.Iface{
		Iface: c.name,
	}
	addr, err := c.wifiClient.IPForInterface(ctx, iface)
	if err != nil {
		return "", errors.Wrap(err, "failed to get the IP address")
	}

	return addr.Ipv4, nil
}
