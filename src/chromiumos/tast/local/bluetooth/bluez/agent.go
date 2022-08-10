// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"github.com/godbus/dbus/v5"

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

func (a *Agent) ExportAgentDelegate(agentDelegate AgentDelegate) error {
	if err := a.dbus.Conn().Export(agentDelegate, a.dbus.ObjectPath(), a.dbus.Iface()); err != nil {
		return errors.Wrap(err, "failed to export AgentDelegate as Agent")
	}
	return nil
}

func (a *Agent) ClearExportedAgentDelegate() error {
	if err := a.dbus.Conn().Export(nil, a.dbus.ObjectPath(), a.dbus.Iface()); err != nil {
		return errors.Wrap(err, "failed to clear Agent export")
	}
	return nil
}

// AgentDelegate handles D-Bus Agent methods (TODO better comment).
type AgentDelegate interface {
	// Released() *dbus.Error
	// RequestPinCode(devicePath dbus.ObjectPath) (string, *dbus.Error)
	// DisplayPinCode(devicePath dbus.ObjectPath, pinCode string) *dbus.Error

	// RequestPassKey todo
	RequestPassKey(devicePath dbus.ObjectPath) (string, *dbus.Error)

	// DisplayPassKey(devicePath dbus.ObjectPath, passKey, entered int) *dbus.Error
	// RequestConfirmation(devicePath dbus.ObjectPath) (string, *dbus.Error)
	// RequestAuthorization(devicePath dbus.ObjectPath) (string, *dbus.Error)=

	// AuthorizeService todo
	AuthorizeService(devicePath dbus.ObjectPath, UUID string) (bool, *dbus.Error)
	//Cancel() *dbus.Error
}

// SimplePinAgentDelegate todo
type SimplePinAgentDelegate struct {
	ctx     context.Context
	pinCode string
}

func NewSimplePinAgentDelegate(ctx context.Context, pinCode string) *SimplePinAgentDelegate {
	return &SimplePinAgentDelegate{
		ctx:     ctx,
		pinCode: pinCode,
	}
}

//
//func (s *SimplePinAgentDelegate) Released() *dbus.Error {
//	testing.ContextLog(s.ctx, "called Released")
//	return nil
//}
//
//func (s *SimplePinAgentDelegate) RequestPinCode(devicePath dbus.ObjectPath) (string, *dbus.Error) {
//	testing.ContextLogf(s.ctx, "called RequestPinCode(%s)", devicePath)
//	return s.pinCode, nil
//}
//
//func (s *SimplePinAgentDelegate) DisplayPinCode(devicePath dbus.ObjectPath, pinCode string) *dbus.Error {
//	testing.ContextLogf(s.ctx, "called DisplayPinCode(%s,%s)", devicePath, pinCode)
//	return nil
//}

func (s *SimplePinAgentDelegate) RequestPassKey(devicePath dbus.ObjectPath) (string, *dbus.Error) {
	testing.ContextLogf(s.ctx, "called RequestPassKey(%s)", devicePath)
	return "", nil
}

//func (s *SimplePinAgentDelegate) DisplayPassKey(devicePath dbus.ObjectPath, passKey, entered int) *dbus.Error {
//	testing.ContextLogf(s.ctx, "called DisplayPassKey(%s,%d,%d)", passKey, entered)
//	return nil
//}
//
//func (s *SimplePinAgentDelegate) RequestConfirmation(devicePath dbus.ObjectPath) (string, *dbus.Error) {
//	testing.ContextLogf(s.ctx, "called RequestConfirmation(%s)", devicePath)
//	return "", nil
//}
//
//func (s *SimplePinAgentDelegate) RequestAuthorization(devicePath dbus.ObjectPath) (string, *dbus.Error) {
//	testing.ContextLogf(s.ctx, "called RequestAuthorization(%s)", devicePath)
//	return "", nil
//}

func (s *SimplePinAgentDelegate) AuthorizeService(devicePath dbus.ObjectPath, UUID string) (bool, *dbus.Error) {
	testing.ContextLogf(s.ctx, "called AuthorizeService(%s,%s)", devicePath, UUID)
	return true, nil
}

//func (s *SimplePinAgentDelegate) Cancel() *dbus.Error {
//	testing.ContextLog(s.ctx, "called Cancel")
//	return nil
//}
