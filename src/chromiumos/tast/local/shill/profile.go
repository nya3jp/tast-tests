// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusProfileInterface = "org.chromium.flimflam.Profile"
)

// Profile property names.
const (
	ProfilePropertyCheckPortalList           = "CheckPortalList"
	ProfilePropertyEntries                   = "Entries"
	ProfilePropertyName                      = "Name"
	ProfilePropertyOfflineMode               = "OfflineMode"
	ProfilePropertyPortalURL                 = "PortalURL"
	ProfilePropertyPortalCheckInterval       = "PortalCheckInterval"
	ProfilePropertyServices                  = "Services"
	ProfilePropertyUserHash                  = "UserHash"
	ProfilePropertyProhibitedTechnologies    = "ProhibitedTechnologies"
	ProfilePropertyArpGateway                = "ArpGateway"
	ProfilePropertyLinkMonitorTechnologies   = "LinkMonitorTechnologies"
	ProfilePropertyNoAutoConnectTechnologies = "NoAutoConnectTechnologies"
)

// Profile entry property names.
const (
	ProfileEntryPropertyName = "Name"
)

// Profile wraps a Profile D-Bus object in shill.
type Profile struct {
	dbusObject *DBusObject
	path       dbus.ObjectPath
	props      *Properties
}

// NewProfile connects to a profile in Shill.
func NewProfile(ctx context.Context, path dbus.ObjectPath) (*Profile, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}

	dbusObj := &DBusObject{iface: dbusProfileInterface, obj: obj, conn: conn}
	props, err := NewProperties(ctx, dbusObj)
	if err != nil {
		return nil, err
	}
	return &Profile{dbusObject: dbusObj, path: path, props: props}, nil
}

// Properties returns existing properties.
func (p *Profile) Properties() *Properties {
	return p.props
}

// String returns the path of the profile.
// It is so named to conform to the Stringer interface.
func (p *Profile) String() string {
	return p.dbusObject.String()
}

// GetProperties refreshes and returns properties.
func (p *Profile) GetProperties(ctx context.Context) (*Properties, error) {
	props, err := NewProperties(ctx, p.dbusObject)
	if err != nil {
		return nil, err
	}
	p.props = props
	return props, nil
}

// SetProperty sets a property to the given value.
func (p *Profile) SetProperty(ctx context.Context, property string, val interface{}) error {
	return p.props.SetProperty(ctx, property, val)
}

// GetEntry calls the GetEntry method on the profile.
func (p *Profile) GetEntry(ctx context.Context, entryID string) (map[string]interface{}, error) {
	var entryProps map[string]interface{}
	if err := p.dbusObject.Call(ctx, "GetEntry", entryID).Store(&entryProps); err != nil {
		return nil, errors.Wrapf(err, "failed to get entry %s", entryID)
	}
	return entryProps, nil
}

// DeleteEntry calls the DeleteEntry method on the profile.
func (p *Profile) DeleteEntry(ctx context.Context, entryID string) error {
	return p.dbusObject.Call(ctx, "DeleteEntry", entryID).Err
}
