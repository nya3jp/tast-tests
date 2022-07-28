// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devices

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
)

// ChameleonDeviceFlow is an interface for making RPC calls to a chameleond
// daemon targeting a specific chameleon device flow.
//
// This is based off of the Python class "chameleond.devices.chameleon_device.Flow"
// from the chameleon source. Refer to that source for more complete
// documentation.
type ChameleonDeviceFlow interface {
	ChameleonDevice

	// IsPhysicalPlugged calls the Chameleond RPC method of the same name.
	// Returns true if the physical cable is plugged.
	IsPhysicalPlugged(ctx context.Context) (bool, error)

	// IsPlugged calls the Chameleond RPC method of the same name.
	// Returns true if the flow is plugged.
	IsPlugged(ctx context.Context) (bool, error)

	// Plug calls the Chameleond RPC method of the same name.
	// Emulates plug.
	Plug(ctx context.Context) error

	// Unplug calls the Chameleond RPC method of the same name.
	// Emulates unplug.
	Unplug(ctx context.Context) error

	// Select calls the Chameleond RPC method of the same name.
	// Selects the flow.
	Select(ctx context.Context) error

	// DoFSM calls the Chameleond RPC method of the same name.
	// Does the Finite-State-Machine to ensure the input flow is ready.
	DoFSM(ctx context.Context) error

	// GetConnectorType calls the Chameleond RPC method of the same name.
	// Returns the human-readable string name for the connector type.
	GetConnectorType(ctx context.Context) (string, error)
}

// CommonChameleonDeviceFlow is a base implementation of ChameleonDeviceFlow that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the ChameleonDeviceFlow interface for more detailed documentation.
type CommonChameleonDeviceFlow struct {
	CommonChameleonDevice
}

// NewCommonChameleonDeviceFlow creates a new instance of
// CommonChameleonDeviceFlow.
func NewCommonChameleonDeviceFlow(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonChameleonDeviceFlow {
	return &CommonChameleonDeviceFlow{
		CommonChameleonDevice: *NewCommonChameleonDevice(xmlrpcClient, methodNamePrefix),
	}
}

// IsPhysicalPlugged calls the Chameleond RPC method of the same name.
// This implements ChameleonDeviceFlow.IsPhysicalPlugged, see that for more
// details.
func (c *CommonChameleonDeviceFlow) IsPhysicalPlugged(ctx context.Context) (bool, error) {
	return c.RPC("IsPhysicalPlugged").CallForBool(ctx)
}

// IsPlugged calls the Chameleond RPC method of the same name.
// This implements ChameleonDeviceFlow.IsPlugged, see that for more details.
func (c *CommonChameleonDeviceFlow) IsPlugged(ctx context.Context) (bool, error) {
	return c.RPC("IsPlugged").CallForBool(ctx)
}

// Plug calls the Chameleond RPC method of the same name.
// This implements ChameleonDeviceFlow.Plug, see that for more details.
func (c *CommonChameleonDeviceFlow) Plug(ctx context.Context) error {
	return c.RPC("Plug").Call(ctx)
}

// Unplug calls the Chameleond RPC method of the same name.
// This implements ChameleonDeviceFlow.Unplug, see that for more details.
func (c *CommonChameleonDeviceFlow) Unplug(ctx context.Context) error {
	return c.RPC("Unplug").Call(ctx)
}

// Select calls the Chameleond RPC method of the same name.
// This implements ChameleonDeviceFlow.Select, see that for more details.
func (c *CommonChameleonDeviceFlow) Select(ctx context.Context) error {
	return c.RPC("Select").Call(ctx)
}

// DoFSM calls the Chameleond RPC method of the same name.
// This implements ChameleonDeviceFlow.DoFSM, see that for more details.
func (c *CommonChameleonDeviceFlow) DoFSM(ctx context.Context) error {
	return c.RPC("DoFSM").Call(ctx)
}

// GetConnectorType calls the Chameleond RPC method of the same name.
// This implements ChameleonDeviceFlow.GetConnectorType, see that for more
// details.
func (c *CommonChameleonDeviceFlow) GetConnectorType(ctx context.Context) (string, error) {
	return c.RPC("GetConnectorType").CallForString(ctx)
}
