// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
)

// MousePeripheral is an interface for making RPC calls to a chameleond daemon
// targeting a specific bluetooth mouse peripheral chameleon device flow.
//
// This is based off of the Python class "chameleond.devices.raspi_bluetooth_flow.RaspiHIDMouse"
// from the chameleon source. Refer to that source for more complete
// documentation.
type MousePeripheral interface {
	BluezPeripheral
	// Move calls the Chameleond RPC method of the same name.
	// Moves the mouse (deltaX, deltaY) steps.
	//
	// If buttons are being pressed, they will stay pressed during this operation.
	// This move is relative to the current position by the HID standard.
	// Valid step values must be in the range [-127,127].
	Move(ctx context.Context, deltaX, deltaY int) error

	// LeftClick calls the Chameleond RPC method of the same name.
	// Makes a left click.
	LeftClick(ctx context.Context) error

	// RightClick calls the Chameleond RPC method of the same name.
	// Make a right click.
	RightClick(ctx context.Context) error

	// ClickAndDrag calls the Chameleond RPC method of the same name.
	// Preforms a left click, drag (delta_x, delta_y) steps, and then the left
	// click is released.
	//
	// This move is relative to the current position by the HID standard.
	// Valid step values must be in the range [-127,127].
	ClickAndDrag(ctx context.Context, deltaX, deltaY int) error

	// Scroll calls the Chameleond RPC method of the same name.
	// Scrolls the mouse wheel steps number of steps.
	//
	// Buttons currently pressed will stay pressed during this operation.
	// Valid step values must be in the range [-127,127].
	//
	// With traditional scrolling negative values scroll down and positive values
	// scroll up. With reversed (formerly "Australian") scrolling, this is
	// reversed.
	Scroll(ctx context.Context, steps int) error
}

// CommonMousePeripheral is a base implementation of MousePeripheral that
// provides methods for making XMLRPC calls to a chameleond daemon.
// See the MousePeripheral interface for more detailed documentation.
type CommonMousePeripheral struct {
	xmlrpc.CommonRPCInterface
	CommonBluezPeripheral
}

// NewCommonMousePeripheral creates a new instance of CommonMousePeripheral.
func NewCommonMousePeripheral(xmlrpcClient *xmlrpc.XMLRpc, methodNamePrefix string) *CommonMousePeripheral {
	return &CommonMousePeripheral{
		CommonBluezPeripheral: *NewCommonBluezPeripheral(xmlrpcClient, methodNamePrefix),
	}
}

// Move calls the Chameleond RPC method of the same name.
// This implements MousePeripheral.Move, see that for more details.
func (c *CommonMousePeripheral) Move(ctx context.Context, deltaX, deltaY int) error {
	return c.RPC("Move").Args(deltaX, deltaY).Call(ctx)
}

// LeftClick calls the Chameleond RPC method of the same name.
// This implements MousePeripheral.LeftClick, see that for more details.
func (c *CommonMousePeripheral) LeftClick(ctx context.Context) error {
	return c.RPC("LeftClick").Call(ctx)
}

// RightClick calls the Chameleond RPC method of the same name.
// This implements MousePeripheral.RightClick, see that for more details.
func (c *CommonMousePeripheral) RightClick(ctx context.Context) error {
	return c.RPC("RightClick").Call(ctx)
}

// ClickAndDrag calls the Chameleond RPC method of the same name.
// This implements MousePeripheral.ClickAndDrag, see that for more details.
func (c *CommonMousePeripheral) ClickAndDrag(ctx context.Context, deltaX, deltaY int) error {
	return c.RPC("ClickAndDrag").Args(deltaX, deltaY).Call(ctx)
}

// Scroll calls the Chameleond RPC method of the same name.
// This implements MousePeripheral.Scroll, see that for more details.
func (c *CommonMousePeripheral) Scroll(ctx context.Context, steps int) error {
	return c.RPC("Scroll").Args(steps).Call(ctx)
}
