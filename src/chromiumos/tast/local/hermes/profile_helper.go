// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hermes

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Profile wraps a Hermes.Profile DBus object.
type Profile struct {
	*dbusutil.DBusObject
}

// NewProfile returns a Profile corresponding to a DBus object at profilePath
func NewProfile(ctx context.Context, profilePath dbus.ObjectPath) (*Profile, error) {
	obj, err := dbusutil.NewDBusObject(ctx, hermesconst.DBusHermesService, hermesconst.DBusHermesProfileInterface, profilePath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get dbus object for profile")
	}
	return &Profile{obj}, nil
}

// IsTestProfile returns true if a profile is a test profile.
func (p *Profile) IsTestProfile(ctx context.Context) (bool, error) {
	props, err := dbusutil.NewDBusProperties(ctx, p.DBusObject)
	if err != nil {
		return false, errors.Wrap(err, "failed to read profile properties")
	}
	class, err := props.GetInt32(hermesconst.ProfilePropertyClass)
	if err != nil {
		return false, errors.Wrap(err, "failed to read profile class")
	}
	return class == hermesconst.ProfileClassTest, nil
}
