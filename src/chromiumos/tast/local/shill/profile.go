// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
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
	PropertyHolder
}

// NewProfile connects to a profile in Shill.
func NewProfile(ctx context.Context, path dbus.ObjectPath) (*Profile, error) {
	ph, err := NewPropertyHolder(ctx, dbusProfileInterface, path)
	if err != nil {
		return nil, err
	}
	return &Profile{PropertyHolder: ph}, nil
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
