// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hermes

import (
	"context"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Euicc wraps a Hermes.Euicc DBus object.
type Euicc struct {
	*dbusutil.DBusObject
}

// GetEuicc returns a DBusObject for the euiccNum'th eSIM.
func GetEuicc(ctx context.Context, euiccNum int) (*Euicc, error) {
	h, err := GetHermesManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Hermes Manager DBus object")
	}

	props, err := dbusutil.NewDBusProperties(ctx, h.DBusObject)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Hermes manager properties")
	}
	euiccPaths, err := props.GetObjectPaths(hermesconst.ManagerPropertyAvailableEuiccs)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get available euiccs")
	}
	if euiccNum >= len(euiccPaths) {
		return nil, errors.Errorf("only %d eSIM's available, cannot find eSIM number %d", len(euiccPaths), euiccNum)
	}
	obj, err := dbusutil.NewDBusObject(ctx, DBusHermesService, DBusHermesEuiccInterface, euiccPaths[euiccNum])
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Euicc")
	}
	return &Euicc{obj}, nil
}

// GetInstalledProfiles reads the eSIM, and returns Profile DBus objects.
func (e *Euicc) GetInstalledProfiles(ctx context.Context) ([]Profile, error) {
	if err := e.DBusObject.Call(ctx, "RequestInstalledProfiles").Err; err != nil {
		return nil, errors.Wrap(err, "unable to request installed profiles")
	}
	props, err := dbusutil.NewDBusProperties(ctx, e.DBusObject)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get euicc properties")
	}
	profilePaths, err := props.GetObjectPaths("InstalledProfiles")
	if err != nil {
		return nil, errors.Wrap(err, "unable to get installed profiles")
	}
	profiles := make([]Profile, len(profilePaths))
	for i, profilePath := range profilePaths {
		obj, err := dbusutil.NewDBusObject(ctx, DBusHermesService, DBusHermesProfileInterface, profilePath)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get dbus object for profile")
		}
		profiles[i] = Profile{obj}
	}
	return profiles, nil
}

// GetEnabledProfile reads the eSIM, and returns the currently enabled Profile.
func (e *Euicc) GetEnabledProfile(ctx context.Context) (Profile, error) {
	profiles, err := e.GetInstalledProfiles(ctx)
	if err != nil {
		return Profile{nil}, errors.Wrap(err, "failed to get installed profiles")
	}

	for _, profile := range profiles {
		props, err := dbusutil.NewDBusProperties(ctx, profile.DBusObject)
		if err != nil {
			return Profile{nil}, errors.Wrap(err, "unable to get profile properties")
		}
		state, err := props.GetInt32(hermesconst.ProfilePropertyState)
		if err != nil {
			return Profile{nil}, errors.Wrap(err, "failed to read profiles state")
		}
		if state == hermesconst.ProfileStateEnabled {
			return profile, nil
		}
	}
	return Profile{nil}, nil
}
