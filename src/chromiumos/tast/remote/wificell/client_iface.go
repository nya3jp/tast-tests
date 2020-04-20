// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/services/cros/network"
)

// ClientIface is interface for the client.
type ClientIface struct {
	name string
	dut  *dut.DUT
}

// NewClientInterface creates a ClinetIface that contains the queryable address properties of a network device.
func (tf *TestFixture) NewClientInterface(ctx context.Context, st shill.Technology) (*ClientIface, error) {
	tech := &network.Technology{
		Technology: string(st),
	}
	netIf, err := tf.wifiClient.Interface(ctx, tech)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the Wifi interface name")
	}
	return &ClientIface{name: netIf.Iface, dut: tf.dut}, nil
}

// Addresses returns the addresses (MAC, IP) associated with interface.
func (c *ClientIface) Addresses(ctx context.Context) (*ip.AddInfo, error) {
	ipr := ip.NewRunner(c.dut)

	return ipr.Addresses(ctx, c.name)
}
