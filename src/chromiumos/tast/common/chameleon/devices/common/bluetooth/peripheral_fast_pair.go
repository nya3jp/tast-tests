// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
)

// FastPairPeripheral is an interface for making RPC calls to a chameleond daemon
// targeting a specific bluetooth fast pair peripheral chameleon device flow.
//
// This is based off of the Python class "chameleond.devices.raspi_bluetooth_flow.RaspiBLEFastPair"
// from the chameleon source. Refer to that source for more complete
// documentation.
type FastPairPeripheral interface {
	BluezPeripheral

	// SetAntispoofingKeyPem calls the Chameleond RPC method of the same name.
	// Sets the anti-spoofing key for the Fast Pair GATT service.
	SetAntispoofingKeyPem(ctx context.Context, keyPem string) error

	// AddAccountKey calls the Chameleond RPC method of the same name.
	// Adds an account key to the Fast Pair GATT service.
	AddAccountKey(ctx context.Context, accountKey string) error
}

// CommonFastPairPeripheral is a base implementation of FastPairPeripheral that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the FastPairPeripheral interface for more detailed documentation.
type CommonFastPairPeripheral struct {
	CommonBluezPeripheral
}

// NewCommonFastPairPeripheral creates a new instance of
// CommonFastPairPeripheral.
func NewCommonFastPairPeripheral(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonFastPairPeripheral {
	return &CommonFastPairPeripheral{
		CommonBluezPeripheral: *NewCommonBluezPeripheral(xmlrpcClient, methodNamePrefix),
	}
}

// SetAntispoofingKeyPem calls the Chameleond RPC method of the same name.
// This implements FastPairPeripheral.SetAntispoofingKeyPem, see that for more
// details.
func (c *CommonFastPairPeripheral) SetAntispoofingKeyPem(ctx context.Context, keyPem string) error {
	return c.RPC("SetAntispoofingKeyPem").Args(keyPem).Call(ctx)
}

// AddAccountKey calls the Chameleond RPC method of the same name.
// This implements FastPairPeripheral.AddAccountKey, see that for more details.
func (c *CommonFastPairPeripheral) AddAccountKey(ctx context.Context, accountKey string) error {
	return c.RPC("AddAccountKey").Args(accountKey).Call(ctx)
}
