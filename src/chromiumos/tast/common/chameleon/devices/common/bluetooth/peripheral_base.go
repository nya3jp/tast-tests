// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/common/chameleon/devices"
	"chromiumos/tast/common/xmlrpc"
)

// BasePeripheral is an interface for making RPC calls to a chameleond daemon
// targeting a specific bluetooth base chameleon device flow.
//
// This is based off of the Python class "chameleond.devices.bluetooth_base_flow.BluetoothBaseFlow"
// from the chameleon source. Refer to that source for more complete
// documentation.
type BasePeripheral interface {
	devices.ChameleonDevice

	// SpecifyDeviceType calls the Chameleond RPC method of the same name.
	// Instantiates the supported device of type deviceType.
	SpecifyDeviceType(ctx context.Context, deviceType string) error

	// EnableServod calls the Chameleond RPC method of the same name.
	// Enables servod for the given board.
	EnableServod(ctx context.Context, board string) (bool, error)
}

// CommonBasePeripheral is a base implementation of BasePeripheral that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the BasePeripheral interface for more detailed documentation.
type CommonBasePeripheral struct {
	devices.CommonChameleonDevice
}

// NewCommonBasePeripheral creates a new instance of CommonBasePeripheral.
func NewCommonBasePeripheral(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonBasePeripheral {
	return &CommonBasePeripheral{
		CommonChameleonDevice: *devices.NewCommonChameleonDevice(xmlrpcClient, methodNamePrefix),
	}
}

// SpecifyDeviceType calls the Chameleond RPC method of the same name.
// This implements BasePeripheral.SpecifyDeviceType, see that for more details.
func (c *CommonBasePeripheral) SpecifyDeviceType(ctx context.Context, deviceType string) error {
	return c.RPC("SpecifyDeviceType").Args(deviceType).Call(ctx)

}

// EnableServod calls the Chameleond RPC method of the same name.
// This implements BasePeripheral.EnableServod, see that for more details.
func (c *CommonBasePeripheral) EnableServod(ctx context.Context, board string) (bool, error) {
	return c.RPC("EnableServod").Args(board).CallForBool(ctx)
}
