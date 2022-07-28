// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
)

// KeyboardPeripheral is an interface for making RPC calls to a chameleond daemon
// targeting a specific bluetooth keyboard peripheral chameleon device flow.
//
// This is based off of the Python class "chameleond.devices.raspi_bluetooth_flow.RaspiHIDKeyboard"
// from the chameleon source. Refer to that source for more complete
// documentation.
type KeyboardPeripheral interface {
	BluezPeripheral

	// KeyboardSendTrace calls the Chameleond RPC method of the same name.
	// Sends scan codes from a data trace over the BT link.
	KeyboardSendTrace(ctx context.Context, inputScanCodes []int) error

	// KeyboardSendString calls the Chameleond RPC method of the same name.
	// Sends characters one-by-one over the BT link.
	KeyboardSendString(ctx context.Context, stringToSend string) error
}

// CommonKeyboardPeripheral is a base implementation of KeyboardPeripheral that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the KeyboardPeripheral interface for more detailed documentation.
type CommonKeyboardPeripheral struct {
	CommonBluezPeripheral
}

// NewCommonKeyboardPeripheral creates a new instance of
// CommonKeyboardPeripheral.
func NewCommonKeyboardPeripheral(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonKeyboardPeripheral {
	return &CommonKeyboardPeripheral{
		CommonBluezPeripheral: *NewCommonBluezPeripheral(xmlrpcClient, methodNamePrefix),
	}
}

// KeyboardSendTrace calls the Chameleond RPC method of the same name.
// This implements KeyboardPeripheral.KeyboardSendTrace, see that for more
// details.
func (c *CommonKeyboardPeripheral) KeyboardSendTrace(ctx context.Context, inputScanCodes []int) error {
	return c.RPC("KeyboardSendTrace").Args(inputScanCodes).Call(ctx)
}

// KeyboardSendString calls the Chameleond RPC method of the same name.
// This implements KeyboardPeripheral.KeyboardSendString, see that for more
// details.
func (c *CommonKeyboardPeripheral) KeyboardSendString(ctx context.Context, stringToSend string) error {
	return c.RPC("KeyboardSendString").Args(stringToSend).Call(ctx)
}
