// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devices

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
)

// ChameleonDevice is an interface for making RPC calls to a chameleond daemon
// targeting a specific chameleon device.
//
// This is based off of the Python class "chameleond.devices.chameleon_device.ChameleonDevice"
// from the chameleon source. Refer to that source for more complete
// documentation.
type ChameleonDevice interface {
	xmlrpc.RPCInterface

	// IsDetected calls the Chameleond RPC method of the same name.
	// Returns true the device can be detected.
	IsDetected(ctx context.Context) (bool, error)

	// InitDevice calls the Chameleond RPC method of the same name.
	// Initializes the real device of chameleon board.
	InitDevice(ctx context.Context) error

	// Reset calls the Chameleond RPC method of the same name.
	// Resets the chameleon device.
	Reset(ctx context.Context) error

	// GetDeviceName calls the Chameleond RPC method of the same name.
	// Returns the human-readable string name for the device.
	GetDeviceName(ctx context.Context) (string, error)
}

// CommonChameleonDevice is a base implementation of ChameleonDevice that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the ChameleonDevice interface for more detailed documentation.
type CommonChameleonDevice struct {
	xmlrpc.CommonRPCInterface
}

// NewCommonChameleonDevice creates a new instance of CommonChameleonDevice.
func NewCommonChameleonDevice(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonChameleonDevice {
	return &CommonChameleonDevice{
		CommonRPCInterface: *xmlrpc.NewCommonRPCInterface(xmlrpcClient, methodNamePrefix),
	}
}

// IsDetected calls the Chameleond RPC method of the same name.
// This implements ChameleonDevice.IsDetected, see that for more details.
func (c *CommonChameleonDevice) IsDetected(ctx context.Context) (bool, error) {
	return c.RPC("IsDetected").CallForBool(ctx)
}

// InitDevice calls the Chameleond RPC method of the same name.
// This implements ChameleonDevice.InitDevice, see that for more details.
func (c *CommonChameleonDevice) InitDevice(ctx context.Context) error {
	return c.RPC("InitDevice").Call(ctx)
}

// Reset calls the Chameleond RPC method of the same name.
// This implements ChameleonDevice.Reset, see that for more details.
func (c *CommonChameleonDevice) Reset(ctx context.Context) error {
	return c.RPC("Reset").Call(ctx)
}

// GetDeviceName calls the Chameleond RPC method of the same name.
// This implements ChameleonDevice.GetDeviceName, see that for more details.
func (c *CommonChameleonDevice) GetDeviceName(ctx context.Context) (string, error) {
	return c.RPC("GetDeviceName").CallForString(ctx)
}
