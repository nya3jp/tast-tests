// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hermes

import (
	"context"
	"strconv"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// EUICC wraps a Hermes.EUICC DBus object.
type EUICC struct {
	*dbusutil.DBusObject
}

// NewEUICC returns a DBusObject for the euiccNum'th (zero based) eSIM.
func NewEUICC(ctx context.Context, euiccNum int) (*EUICC, error) {
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
	obj, err := dbusutil.NewDBusObject(ctx, hermesconst.DBusHermesService, hermesconst.DBusHermesEuiccInterface, euiccPaths[euiccNum])
	if err != nil {
		return nil, errors.Wrap(err, "unable to get EUICC object")
	}
	return &EUICC{obj}, nil
}

// InstalledProfiles reads the eSIM, and returns the installed profiles in the eSIM.
func (e *EUICC) InstalledProfiles(ctx context.Context, shouldNotSwitchSlot bool) ([]Profile, error) {
	if err := e.Call(ctx, "RefreshInstalledProfiles", shouldNotSwitchSlot).Err; err != nil {
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
		obj, err := dbusutil.NewDBusObject(ctx, hermesconst.DBusHermesService, hermesconst.DBusHermesProfileInterface, profilePath)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get dbus object for profile")
		}
		profiles[i] = Profile{obj}
	}
	return profiles, nil
}

// EnabledProfile reads the eSIM, and returns the enabled Profile of the eSIM if found.
func (e *EUICC) EnabledProfile(ctx context.Context) (*Profile, error) {
	profiles, err := e.InstalledProfiles(ctx, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get installed profiles")
	}

	for _, profile := range profiles {
		props, err := dbusutil.NewDBusProperties(ctx, profile.DBusObject)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get profile properties")
		}
		state, err := props.GetInt32(hermesconst.ProfilePropertyState)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read profiles state")
		}
		if state == hermesconst.ProfileStateEnabled {
			return &profile, nil
		}
	}
	return nil, nil
}

// GetEUICC will return a EUICC dbus object and its slot number. If findTestEuicc is set, a test eUICC will be returned, else a prod eUICC will be returned.
func GetEUICC(ctx context.Context, findTestEuicc bool) (*EUICC, int, error) {
	h, err := GetHermesManager(ctx)
	if err != nil {
		return nil, -1, errors.Wrap(err, "could not get Hermes Manager DBus object")
	}

	props, err := dbusutil.NewDBusProperties(ctx, h.DBusObject)
	if err != nil {
		return nil, -1, errors.Wrap(err, "unable to get Hermes manager properties")
	}
	euiccPaths, err := props.GetObjectPaths(hermesconst.ManagerPropertyAvailableEuiccs)
	if err != nil {
		return nil, -1, errors.Wrap(err, "unable to get available euiccs")
	}

	euiccType := "prod"
	if findTestEuicc {
		euiccType = "test"
	}

	for _, euiccPath := range euiccPaths {
		obj, err := dbusutil.NewDBusObject(ctx, hermesconst.DBusHermesService, hermesconst.DBusHermesEuiccInterface, euiccPath)
		if err != nil {
			return nil, -1, errors.Wrap(err, "unable to get EUICC object")
		}
		response := obj.Call(ctx, "IsTestEuicc")
		if response.Err != nil || len(response.Body) != 1 {
			continue
		}
		if isTestEuicc, ok := response.Body[0].(bool); !ok || isTestEuicc != findTestEuicc {
			continue
		}

		testing.ContextLogf(ctx, "Found %s EUICC on path: %s", euiccType, euiccPath)
		slot, err := strconv.Atoi(string(euiccPath)[len(string(euiccPath))-1:])
		if err != nil {
			return nil, -1, errors.Wrap(err, "couldn't get euicc slot number")
		}
		return &EUICC{obj}, slot, nil
	}

	return nil, -1, errors.Wrapf(err, "no %s euicc found", euiccType)
}
