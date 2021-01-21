// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package modemmanager

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusModemmanagerPath           = "/org/freedesktop/ModemManager1"
	dbusModemmanagerService        = "org.freedesktop.ModemManager1"
	dbusModemmanagerModemInterface = "org.freedesktop.ModemManager1.Modem"
	dbusModemmanagerSimInterface   = "org.freedesktop.ModemManager1.Sim"
)

// Modem wraps a Modemmanager.Modem D-Bus object.
type Modem struct {
	*dbusutil.PropertyHolder
}

// NewModem creates a new PropertyHolder instance for the Modem object.
func NewModem(ctx context.Context) (*Modem, error) {
	_, obj, err := dbusutil.ConnectNoTiming(ctx, dbusModemmanagerService, dbusModemmanagerPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to service %s", dbusModemmanagerService)
	}
	managed, err := dbusutil.ManagedObjects(ctx, obj)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ManagedObjects")
	}
	modemPath := dbus.ObjectPath("")
	for iface, paths := range managed {
		if iface == dbusModemmanagerModemInterface {
			if len(paths) > 0 {
				modemPath = paths[0]
			}
			break
		}
	}
	if modemPath == dbus.ObjectPath("") {
		return nil, errors.Wrap(err, "failed to get Modem path")
	}
	ph, err := dbusutil.NewPropertyHolder(ctx, dbusModemmanagerService, dbusModemmanagerModemInterface, modemPath)
	if err != nil {
		return nil, err
	}
	return &Modem{ph}, nil
}

// GetSimProperties creates a PropertyHolder for the Sim object and returns the associated Properties.
func (m *Modem) GetSimProperties(ctx context.Context, simPath dbus.ObjectPath) (*dbusutil.Properties, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, dbusModemmanagerService, dbusModemmanagerSimInterface, simPath)
	if err != nil {
		return nil, err
	}
	return ph.GetDBusProperties(ctx)
}
