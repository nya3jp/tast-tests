// Copyright 2021 The ChromiumOS Authors
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

// ActivationCode returns the profile's activation code.
func (p *Profile) ActivationCode(ctx context.Context) (string, error) {
	return p.getStringProperty(ctx, hermesconst.ProfilePropertyActivationCode)
}

// Iccid returns the profile's ICCID.
func (p *Profile) Iccid(ctx context.Context) (string, error) {
	return p.getStringProperty(ctx, hermesconst.ProfilePropertyIccid)
}

// Nickname returns the profile's nickname.
func (p *Profile) Nickname(ctx context.Context) (string, error) {
	return p.getStringProperty(ctx, hermesconst.ProfilePropertyNickname)
}

// ServiceProvider returns the profile's network service provider.
func (p *Profile) ServiceProvider(ctx context.Context) (string, error) {
	return p.getStringProperty(ctx, hermesconst.ProfilePropertyServiceProvider)
}

// Rename changes the profile's nickname.
func (p *Profile) Rename(ctx context.Context, nickName string) error {
	if err := p.Call(ctx, hermesconst.ProfileMethodRename, nickName).Err; err != nil {
		return errors.Wrap(err, "failed to rename profile")
	}
	return nil
}

func (p *Profile) getStringProperty(ctx context.Context, propertyName string) (string, error) {
	props, err := dbusutil.NewDBusProperties(ctx, p.DBusObject)
	if err != nil {
		return "", errors.Wrap(err, "failed to read profile properties")
	}
	value, err := props.GetString(propertyName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read profile property %s", propertyName)
	}
	return value, nil
}
