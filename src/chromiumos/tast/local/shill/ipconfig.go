// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

const (
	dbusIPConfigInterface = "org.chromium.flimflam.IPConfig"
)

// IPConfig wraps an IPConfig D-Bus object in shill.
type IPConfig struct {
	*dbusutil.PropertyHolder
}

// NewIPConfig connects to an IPConfig in Shill.
func NewIPConfig(ctx context.Context, path dbus.ObjectPath) (*IPConfig, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, dbusService, dbusIPConfigInterface, path)
	if err != nil {
		return nil, err
	}
	return &IPConfig{PropertyHolder: ph}, nil
}
