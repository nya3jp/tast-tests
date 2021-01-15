// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package modemmanager

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// Modem wraps a Modemmanager.Modem D-Bus object.
type Modem struct {
	*dbusutil.PropertyHolder
}

// NewModem creates a new PropertyHolder instance for the Modem object.
func NewModem(ctx context.Context) (*Modem, error) {
	_, obj, err := dbusutil.ConnectNoTiming(ctx, DBusModemmanagerService, DBusModemmanagerPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to service %s", DBusModemmanagerService)
	}

	// It may take 30+ seconds for a Modem object to appear after an Inhibit or
	// a reset, so poll the managed objects for 60 seconds looking for a Modem.
	modemPath := dbus.ObjectPath("")
	if err := testing.Poll(ctx, func(ctx context.Context) (e error) {
		managed, err := dbusutil.ManagedObjects(ctx, obj)
		if err != nil {
			return errors.Wrap(err, "failed to get ManagedObjects")
		}
		for iface, paths := range managed {
			if iface == DBusModemmanagerModemInterface {
				if len(paths) > 0 {
					modemPath = paths[0]
				}
				break
			}
		}
		if modemPath == dbus.ObjectPath("") {
			return errors.Wrap(err, "failed to get Modem path")
		}
		return nil // success
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		return nil, err
	}
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerModemInterface, modemPath)
	if err != nil {
		return nil, err
	}
	return &Modem{ph}, nil
}

// GetSimProperties creates a PropertyHolder for the Sim object and returns the associated Properties.
func (m *Modem) GetSimProperties(ctx context.Context, simPath dbus.ObjectPath) (*dbusutil.Properties, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, DBusModemmanagerService, DBusModemmanagerSimInterface, simPath)
	if err != nil {
		return nil, err
	}
	return ph.GetDBusProperties(ctx)
}
