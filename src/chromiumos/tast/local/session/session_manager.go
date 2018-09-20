// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"fmt"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	dbusName      = "org.chromium.SessionManager"
	dbusPath      = "/org/chromium/SessionManager"
	dbusInterface = "org.chromium.SessionManagerInterface"
)

type SessionManager struct {
	obj dbus.BusObject
}

func NewSessionManager(ctx context.Context) (*SessionManager, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("Failed connection to system bus: %v", err)
	}

	testing.ContextLogf(ctx, "Waiting for %s D-Bus service", dbusName)
	if err := dbusutil.WaitForService(ctx, conn, dbusName); err != nil {
		return nil, fmt.Errorf("Failed waiting for SessionManager service: %v", err)
	}

	obj := conn.Object(dbusName, dbusPath)
	return &SessionManager{obj}, nil
}

// Calls SessionManager.EnableChromeTesting D-Bus method.
func (m *SessionManager) EnableChromeTesting(
	ctx context.Context,
	forceRelaunch bool,
	extraArguments []string,
	extraEnvironmentVariables []string) (string, error) {
	c := m.callDBus(ctx, "EnableChromeTesting",
		forceRelaunch, extraArguments, extraEnvironmentVariables)
	var filepath string
	if err := c.Store(&filepath); err != nil {
		return "", err
	}
	return filepath, nil
}

// Thin wrapper of CallWithContext for convenience.
func (m *SessionManager) callDBus(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return m.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
