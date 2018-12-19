// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package session interacts with session_manager.
package session

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
	"github.com/shirou/gopsutil/process"

	"chromiumos/policy/enterprise_management"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.SessionManager"
	dbusPath      = "/org/chromium/SessionManager"
	dbusInterface = "org.chromium.SessionManagerInterface"
)

// GetSessionManagerPID returns the PID of the session_manager.
func GetSessionManagerPID() (int, error) {
	const exePath = "/sbin/session_manager"

	all, err := process.Pids()
	if err != nil {
		return -1, err
	}

	for _, pid := range all {
		if proc, err := process.NewProcess(pid); err != nil {
			// Assume that the process exited.
			continue
		} else if exe, err := proc.Exe(); err == nil && exe == exePath {
			return int(pid), nil
		}
	}
	return -1, errors.New("session_manager process not found")
}

// SessionManager is used to interact with the session_manager process over
// D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/login_manager/dbus_bindings/org.chromium.SessionManagerInterface.xml
type SessionManager struct { // NOLINT
	conn *dbus.Conn
	obj  dbus.BusObject
}

// NewSessionManager connects to session_manager via D-Bus and returns a SessionManager object.
func NewSessionManager(ctx context.Context) (*SessionManager, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &SessionManager{conn, obj}, nil
}

// EnableChromeTesting calls SessionManager.EnableChromeTesting D-Bus method.
func (m *SessionManager) EnableChromeTesting(
	ctx context.Context,
	forceRelaunch bool,
	extraArguments []string,
	extraEnvironmentVariables []string) (filepath string, err error) {
	c := m.call(ctx, "EnableChromeTesting",
		forceRelaunch, extraArguments, extraEnvironmentVariables)
	if err := c.Store(&filepath); err != nil {
		return "", err
	}
	return filepath, nil
}

// HandleSupervisedUserCreationStarting calls
// SessionManager.HandleSupervisedUserCreationStarting D-Bus method.
func (m *SessionManager) HandleSupervisedUserCreationStarting(
	ctx context.Context) error {
	return m.call(ctx, "HandleSupervisedUserCreationStarting").Err
}

// HandleSupervisedUserCreationFinished calls
// SessionManager.HandleSupervisedUserCreationFinished D-Bus method.
func (m *SessionManager) HandleSupervisedUserCreationFinished(
	ctx context.Context) error {
	return m.call(ctx, "HandleSupervisedUserCreationFinished").Err
}

// RetrieveSessionState calls SessionManager.RetrieveSessionState D-Bus method.
func (m *SessionManager) RetrieveSessionState(ctx context.Context) (string, error) {
	c := m.call(ctx, "RetrieveSessionState")
	var state string
	if err := c.Store(&state); err != nil {
		return "", err
	}
	return state, nil
}

// StartSession calls SessionManager.StartSession D-Bus method.
func (m *SessionManager) StartSession(ctx context.Context, accountID, uniqueIdentifier string) error {
	return m.call(ctx, "StartSession", accountID, uniqueIdentifier).Err
}

// StorePolicy calls SessionManager.StorePolicy D-Bus method.
func (m *SessionManager) StorePolicy(ctx context.Context, policy *enterprise_management.PolicyFetchResponse) error {
	return m.callProtoMethod(ctx, "StorePolicy", policy, nil)
}

// RetrievePolicy calls SessionManager.RetrievePolicy D-Bus method.
func (m *SessionManager) RetrievePolicy(ctx context.Context) (*enterprise_management.PolicyFetchResponse, error) {
	ret := &enterprise_management.PolicyFetchResponse{}
	if err := m.callProtoMethod(ctx, "RetrievePolicy", nil, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// call is thin wrapper of CallWithContext for convenience.
func (m *SessionManager) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return m.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// callProtoMethod is thin wrapper of CallProtoMethod for convenience.
func (m *SessionManager) callProtoMethod(ctx context.Context, method string, in, out proto.Message) error {
	return dbusutil.CallProtoMethod(ctx, m.obj, dbusInterface+"."+method, in, out)
}

// WatchSessionStateChanged returns a SignalWatcher to observe
// "SessionStateChanged" signal for the given state. If state is empty, it
// matches with any "SessionStateChanged" signals.
func (m *SessionManager) WatchSessionStateChanged(ctx context.Context, state string) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "SessionStateChanged",
		Arg0:      state,
	}
	return dbusutil.NewSignalWatcher(ctx, m.conn, spec)
}

// WatchPropertyChangeComplete returns a SignalWatcher to observe
// "PropertyChangeComplete" signal.
func (m *SessionManager) WatchPropertyChangeComplete(ctx context.Context) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "PropertyChangeComplete",
	}
	return dbusutil.NewSignalWatcher(ctx, m.conn, spec)
}

// WatchSetOwnerKeyComplete returns a SignalWatcher to observe
// "SetOwnerKeyComplete" signal.
func (m *SessionManager) WatchSetOwnerKeyComplete(ctx context.Context) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "SetOwnerKeyComplete",
	}
	return dbusutil.NewSignalWatcher(ctx, m.conn, spec)
}
