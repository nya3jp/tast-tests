// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluez

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/local/dbusutil"
)

// Dbus path constants.
const (
	bluezService                   = "org.bluez"
	bluezAdapterIface              = bluezService + ".Adapter1"
	bluezDeviceIface               = bluezService + ".Device1"
	bluezLEAdvertisingManagerIface = bluezService + ".LEAdvertisingManager1"
	bluezAdminPolicyStatusIface    = bluezService + ".AdminPolicyStatus1"
)

// NewBluezDBusObject creates a new dbusutil.DBusObject with the service
// parameter prefilled as bluezService.
func NewBluezDBusObject(ctx context.Context, objIface string, path dbus.ObjectPath) (*dbusutil.DBusObject, error) {
	return dbusutil.NewDBusObject(ctx, bluezService, objIface, path)
}

func collectExistingBluezObjectPaths(ctx context.Context, objIface string) ([]dbus.ObjectPath, error) {
	_, serviceObj, err := dbusutil.Connect(ctx, bluezService, "/")
	if err != nil {
		return nil, err
	}
	managedObjs, err := dbusutil.ManagedObjects(ctx, serviceObj)
	if err != nil {
		return nil, err
	}
	return managedObjs[objIface], nil
}
