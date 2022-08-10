// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"context"

	"github.com/godbus/dbus/v5"
)

// AgentDelegate processes D-Bus Agent methods of the same name.
type AgentDelegate interface {
	// RequestPinCode is called by D-Bus to retrieve a pin code for the Agent.
	RequestPinCode(devicePath dbus.ObjectPath) (string, *dbus.Error)

	// AuthorizeService is called by D-Bus to check for authorization of the
	// service for the Agent.
	AuthorizeService(devicePath dbus.ObjectPath, UUID string) (bool, *dbus.Error)
}

// SimplePinAgentDelegate is a simple implementation of AgentDelegate that can
// provide a pin code and will always authorize services.
type SimplePinAgentDelegate struct {
	ctx     context.Context
	pinCode string
}

// NewSimplePinAgentDelegate creates a new SimplePinAgentDelegate.
func NewSimplePinAgentDelegate(ctx context.Context, pinCode string) *SimplePinAgentDelegate {
	return &SimplePinAgentDelegate{
		ctx:     ctx,
		pinCode: pinCode,
	}
}

// RequestPinCode implements AgentDelegate.RequestPinCode. It returns the preset
// pin code.
func (s *SimplePinAgentDelegate) RequestPinCode(devicePath dbus.ObjectPath) (string, *dbus.Error) {
	return s.pinCode, nil
}

// AuthorizeService implements AgentDelegate.AuthorizeService. It always returns
// true to always allow the pairing.
func (s *SimplePinAgentDelegate) AuthorizeService(devicePath dbus.ObjectPath, UUID string) (bool, *dbus.Error) {
	return true, nil
}
