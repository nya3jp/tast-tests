// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/local/dbusutil"
)

// Agent is a dbus client for bluez agents.
type Agent struct {
	dbus *dbusutil.DBusObject
}

// NewAgent creates a new bluetooth Agent from the passed D-Bus object path.
func NewAgent(ctx context.Context, path dbus.ObjectPath) (*Agent, error) {
	obj, err := NewBluezDBusObject(ctx, bluezAgentIface, path)
	if err != nil {
		return nil, err
	}
	a := &Agent{
		dbus: obj,
	}
	return a, nil
}

// Agents creates an Agent for all bluetooth agents in the system.
func Agents(ctx context.Context) ([]*Agent, error) {
	paths, err := collectExistingBluezObjectPaths(ctx, bluezAgentIface)
	if err != nil {
		return nil, err
	}
	agents := make([]*Agent, len(paths))
	for i, path := range paths {
		agent, err := NewAgent(ctx, path)
		if err != nil {
			return nil, err
		}
		agents[i] = agent
	}
	return agents, nil
}

// DBusObject returns the D-Bus object wrapper for this object.
func (a *Agent) DBusObject() *dbusutil.DBusObject {
	return a.dbus
}

// AgentDelegate handles D-Bus Agent methods (TODO better comment).
type AgentDelegate interface {
	Released(ctx context.Context) error
	RequestPinCode(ctx context.Context, devicePath dbus.ObjectPath) (int, error)
	DisplayPinCode(ctx context.Context, devicePath dbus.ObjectPath, pinCode int) error
	RequestPassKey(ctx context.Context, devicePath dbus.ObjectPath) (string, error)
	DisplayPassKey(ctx context.Context, devicePath dbus.ObjectPath, passKey, entered int) error
	RequestConfirmation(ctx context.Context, devicePath dbus.ObjectPath) (string, error)
	RequestAuthorization(ctx context.Context, devicePath dbus.ObjectPath) (string, error)
	AuthorizeService(ctx context.Context, devicePath dbus.ObjectPath, UUID string) (string, error)
	Cancel(ctx context.Context) error
}
