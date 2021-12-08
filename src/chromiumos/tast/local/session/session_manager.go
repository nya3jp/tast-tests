// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package session interacts with session_manager.
package session

import (
	"context"
	"time"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	"chromiumos/policy/chromium/policy/enterprise_management_proto"
	lm "chromiumos/system_api/login_manager_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/timing"
)

const (
	dbusName      = "org.chromium.SessionManager"
	dbusPath      = "/org/chromium/SessionManager"
	dbusInterface = "org.chromium.SessionManagerInterface"
)

// callMultiProtoMethod is similar to callProtoMethod, but with multiple input
// arguments. It is not part of proto.go since it's a one-off method and its
// usage should not be encouraged. DBus proto methods should only have one input
// argument that wrap multiple protos if necessary. See discussion in CL:1409564.
func callMultiProtoMethod(ctx context.Context, obj dbus.BusObject, method string, in []proto.Message, out proto.Message) error {
	var args []interface{}

	for index, inProto := range in {
		marshIn, err := proto.Marshal(inProto)
		if err != nil {
			return errors.Wrapf(err, "failed marshaling %s arg at index %d", method, index)
		}
		args = append(args, marshIn)
	}

	call := obj.CallWithContext(ctx, method, 0, args...)
	if call.Err != nil {
		return errors.Wrapf(call.Err, "failed calling %s", method)
	}
	if out != nil {
		var marshOut []byte
		if err := call.Store(&marshOut); err != nil {
			return errors.Wrapf(err, "failed reading %s response", method)
		}
		if err := proto.Unmarshal(marshOut, out); err != nil {
			return errors.Wrapf(err, "failed unmarshaling %s response", method)
		}
	}
	return nil
}

// GetSessionManagerPID returns the PID of the session_manager.
func GetSessionManagerPID() (int, error) {
	p, err := procutil.FindUnique(procutil.ByExe("/sbin/session_manager"))
	if err != nil {
		return -1, errors.Wrap(err, "session_manager process not found")
	}
	return int(p.Pid), nil
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
func (m *SessionManager) EnableChromeTesting(ctx context.Context, forceRelaunch bool,
	extraArguments, extraEnvironmentVariables []string) (filepath string, err error) {
	ctx, st := timing.Start(ctx, "enable_chrome_testing")
	defer st.End()

	c := m.call(ctx, "EnableChromeTesting",
		forceRelaunch, extraArguments, extraEnvironmentVariables)
	if err := c.Store(&filepath); err != nil {
		return "", err
	}
	return filepath, nil
}

// EnableChromeTestingAndWait calls EnableChromeTesting and waits for the new
// chrome process to start up.
func (m *SessionManager) EnableChromeTestingAndWait(ctx context.Context, forceRelaunch bool,
	extraArguments, extraEnvironmentVariables []string) (filepath string, err error) {
	// Wait for a browser to start since session_manager can take a while to start it.
	old, err := ashproc.WaitForRoot(ctx, time.Minute)
	if err != nil {
		return "", errors.Wrap(err, "failed to find the browser process")
	}

	filepath, err = m.EnableChromeTesting(ctx, forceRelaunch, extraArguments, extraEnvironmentVariables)
	if err != nil {
		return "", err
	}

	// Wait for the current Chrome to shut down first.
	if err := procutil.WaitForTerminated(ctx, old, 10*time.Second); err != nil {
		return "", errors.Wrap(err, "browser process didn't terminate")
	}

	// Wait for a new browser to appear.
	if _, err := ashproc.WaitForRoot(ctx, 10*time.Second); err != nil {
		return "", errors.Wrap(err, "chrome is not restarted")
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

// IsScreenLocked calls SessionManager.IsScreenLocked D-Bus method.
func (m *SessionManager) IsScreenLocked(ctx context.Context) (screenLocked bool, err error) {
	c := m.call(ctx, "IsScreenLocked")
	if err := c.Store(&screenLocked); err != nil {
		return false, err
	}
	return screenLocked, nil
}

// LockScreen calls SessionManager.LockScreen D-Bus method.
func (m *SessionManager) LockScreen(ctx context.Context) error {
	return m.call(ctx, "LockScreen").Err
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

// StorePolicyEx calls SessionManager.StorePolicyEx D-Bus method.
func (m *SessionManager) StorePolicyEx(ctx context.Context, descriptor *lm.PolicyDescriptor, policy *enterprise_management_proto.PolicyFetchResponse) error {
	return callMultiProtoMethod(ctx, m.obj, dbusInterface+"."+"StorePolicyEx", []proto.Message{descriptor, policy}, nil)
}

// RetrievePolicyEx calls SessionManager.RetrievePolicyEx D-Bus method.
func (m *SessionManager) RetrievePolicyEx(ctx context.Context, descriptor *lm.PolicyDescriptor) (*enterprise_management_proto.PolicyFetchResponse, error) {
	ret := &enterprise_management_proto.PolicyFetchResponse{}
	if err := m.callProtoMethod(ctx, "RetrievePolicyEx", descriptor, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// RetrieveActiveSessions calls SessionManager.RetrieveActiveSessions D-Bus method.
func (m *SessionManager) RetrieveActiveSessions(ctx context.Context) (map[string]string, error) {
	c := m.call(ctx, "RetrieveActiveSessions")
	var ret map[string]string
	if err := c.Store(&ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// RetrievePrimarySession calls SessionManager.RetrievePrimarySession D-Bus method.
func (m *SessionManager) RetrievePrimarySession(ctx context.Context) (username, hash string, err error) {
	c := m.call(ctx, "RetrievePrimarySession")
	if err := c.Store(&username, &hash); err != nil {
		return "", "", err
	}
	return username, hash, nil
}

// GetArcStartTimeTicks calls SessionManager.GetArcStartTimeTicks D-Bus method.
func (m *SessionManager) GetArcStartTimeTicks(ctx context.Context) (time.Time, error) {
	c := m.call(ctx, "GetArcStartTimeTicks")
	var ret int64
	if err := c.Store(&ret); err != nil {
		return time.Time{}, err
	}
	// The unit of ticks is microsecond.
	return time.Unix(0, ret*1000), nil
}

// call is thin wrapper of CallWithContext for convenience.
func (m *SessionManager) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return m.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// callProtoMethod is thin wrapper of CallProtoMethod for convenience.
func (m *SessionManager) callProtoMethod(ctx context.Context, method string, in, out proto.Message) error {
	return dbusutil.CallProtoMethod(ctx, m.obj, dbusInterface+"."+method, in, out)
}

// WatchScreenIsLocked returns a SignalWatcher to observe the
// "ScreenIsLocked" signal.
func (m *SessionManager) WatchScreenIsLocked(ctx context.Context) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "ScreenIsLocked",
	}
	return dbusutil.NewSignalWatcher(ctx, m.conn, spec)
}

// WatchScreenIsUnlocked returns a SignalWatcher to observe the
// "ScreenIsUnlocked" signal.
func (m *SessionManager) WatchScreenIsUnlocked(ctx context.Context) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "ScreenIsUnlocked",
	}
	return dbusutil.NewSignalWatcher(ctx, m.conn, spec)
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
