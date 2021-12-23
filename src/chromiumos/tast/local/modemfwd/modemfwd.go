// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package modemfwd interacts with modemfwd D-Bus service.
package modemfwd

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusPath      = "/org/chromium/Modemfwd"
	dbusName      = "org.chromium.Modemfwd"
	dbusInterface = "org.chromium.Modemfwd"
	// DisableAutoUpdatePref disables auto update on modemfwd
	DisableAutoUpdatePref = "/var/lib/modemfwd/disable_auto_update"
)

// Modemfwd is used to interact with the modemfwd process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/modemfwd/dbus_bindings/org.chromium.Modemfwd.xml
type Modemfwd struct {
	*dbusutil.DBusObject
}

// New connects to modemfwd via D-Bus and returns a Modemfwd object.
func New(ctx context.Context) (*Modemfwd, error) {
	obj, err := dbusutil.NewDBusObject(ctx, dbusName, dbusInterface, dbusPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to modemfwd")
	}
	return &Modemfwd{obj}, nil
}

// ForceFlash calls modemfwd's ForceFlash D-Bus method.
func (m *Modemfwd) ForceFlash(ctx context.Context, device string, options map[string]string) error {
	result := false
	if err := m.Call(ctx, "ForceFlash", device, options).Store(&result); err != nil {
		return err
	}
	if !result {
		return errors.New("ForceFlash returned false")
	}
	return nil
}
