// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// AgentManager is a dbus client for bluez agent managers.
type AgentManager struct {
	dbus *dbusutil.DBusObject
}

// NewAgentManager creates a new bluetooth AgentManager from the passed D-Bus object path.
func NewAgentManager(ctx context.Context, path dbus.ObjectPath) (*AgentManager, error) {
	obj, err := NewBluezDBusObject(ctx, bluezAgentManagerIface, path)
	if err != nil {
		return nil, err
	}
	a := &AgentManager{
		dbus: obj,
	}
	return a, nil
}

// AgentManagers creates an AgentManager for all bluetooth agent managers
// in the system.
func AgentManagers(ctx context.Context) ([]*AgentManager, error) {
	paths, err := collectExistingBluezObjectPaths(ctx, bluezAgentManagerIface)
	if err != nil {
		return nil, err
	}
	agentManagers := make([]*AgentManager, len(paths))
	for i, path := range paths {
		agentManager, err := NewAgentManager(ctx, path)
		if err != nil {
			return nil, err
		}
		agentManagers[i] = agentManager
	}
	return agentManagers, nil
}

// DBusObject returns the D-Bus object wrapper for this object.
func (am *AgentManager) DBusObject() *dbusutil.DBusObject {
	return am.dbus
}

// RegisterAgent registers the agent at agentPath with the provided input and
// display capability with the remote agent manager.
//
// The agent is used for pairing and for authorization of incoming connection
// requests.
func (am *AgentManager) RegisterAgent(ctx context.Context, agentPath dbus.ObjectPath, capability string) error {
	c := am.dbus.Call(ctx, "RegisterAgent", agentPath, capability)
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to register agent at %q", agentPath)
	}
	return nil
}

// UnregisterAgent unregisters the agent at agentPath with the remote agent
// manager.
func (am *AgentManager) UnregisterAgent(ctx context.Context, agentPath dbus.ObjectPath) error {
	c := am.dbus.Call(ctx, "UnregisterAgent", agentPath)
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to unregister agent at %q", agentPath)
	}
	return nil
}

// RequestDefaultAgent urequests that the agent at agentPath be made the
// default agent.
func (am *AgentManager) RequestDefaultAgent(ctx context.Context, agentPath dbus.ObjectPath) error {
	c := am.dbus.Call(ctx, "RequestDefaultAgent", agentPath)
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to request default agent for agent at %q", agentPath)
	}
	return nil
}
