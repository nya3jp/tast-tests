// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/ip"
)

// ClientIface provides methods to access properties of test client (DUT)'s network interface.
type ClientIface struct {
	name string   // Network interface name.
	dut  *dut.DUT // Gate of the client.
}

// NewClientInterface creates a ClientIface.
func (tf *TestFixture) NewClientInterface(ctx context.Context) (*ClientIface, error) {
	netIf, err := tf.wifiClient.Interface(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi interface name")
	}
	return &ClientIface{name: netIf.Iface, dut: tf.dut}, nil
}

// Addresses returns the addresses (MAC, IP) associated with the interface.
func (c *ClientIface) Addresses(ctx context.Context) (*ip.AddrInfo, error) {
	ipr := ip.NewRunner(c.dut)
	return ipr.Addresses(ctx, c.name)
}
