// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package modemmanager

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

const (
	dbusModemmanagerSimInterface = "org.freedesktop.ModemManager1.Sim"
)

// Sim wraps a Modemmanager.Sim D-Bus object.
type Sim struct {
	*dbusutil.PropertyHolder
}

// NewSim creates a new PropertyHolder instance for the Sim object.
func NewSim(ctx context.Context, simPath dbus.ObjectPath) (*Sim, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, dbusModemmanagerService, dbusModemmanagerSimInterface, simPath)
	if err != nil {
		return nil, err
	}
	return &Sim{ph}, nil
}
