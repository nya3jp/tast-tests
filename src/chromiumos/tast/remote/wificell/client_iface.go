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

// ClientIface is interface for the client.
type ClientIface struct {
	name string
	dut  *dut.DUT
}

// NewClientInterface creates a ClinetIface that contains the queryable address properties of a network device.
func (tf *TestFixture) NewClientInterface(ctx context.Context) (*ClientIface, error) {
	netIf, err := tf.wifiClient.Interface(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi interface name")
	}
	return &ClientIface{name: netIf.Iface, dut: tf.dut}, nil
}

// Addresses returns the addresses (MAC, IP) associated with interface.
func (c *ClientIface) Addresses(ctx context.Context) (*ip.AddInfo, error) {
	ipr := ip.NewRunner(c.dut)
	return ipr.Addresses(ctx, c.name)
}
