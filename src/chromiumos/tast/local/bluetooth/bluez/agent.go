// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Agent is a dbus client for bluez agents.
type Agent struct {
	dbus     *dbusutil.DBusObject
	delegate AgentDelegate
}

// NewAgent creates a new bluetooth Agent from the passed D-Bus object path.
func NewAgent(ctx context.Context, path dbus.ObjectPath) (*Agent, error) {
	if path == "" {
		path = buildNewUniqueObjectPath("/test/agent")
	}
	obj, err := NewBluezDBusObject(ctx, bluezAgentIface, path)
	if err != nil {
		return nil, err
	}
	return &Agent{dbus: obj}, nil
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

// ExportAgentDelegate exports an agentDelegate so that its functions may be
// called by dbus for this Agent.
func (a *Agent) ExportAgentDelegate(agentDelegate AgentDelegate) error {
	if err := a.dbus.Conn().Export(agentDelegate, a.dbus.ObjectPath(), a.dbus.Iface()); err != nil {
		return errors.Wrap(err, "failed to export AgentDelegate as Agent")
	}
	return nil
}

// ClearExportedAgentDelegate clears any AgentDelegate previously set with
// ExportAgentDelegate.
func (a *Agent) ClearExportedAgentDelegate() error {
	if err := a.dbus.Conn().Export(nil, a.dbus.ObjectPath(), a.dbus.Iface()); err != nil {
		return errors.Wrap(err, "failed to clear Agent export")
	}
	return nil
}
