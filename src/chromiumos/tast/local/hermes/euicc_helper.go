// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hermes

import (
	"context"
	"strconv"

	"github.com/godbus/dbus/v5"

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

func (e *EUICC) filterProfiles(ctx context.Context, paths []dbus.ObjectPath, desiredStates []int) ([]Profile, error) {
	var profiles []Profile
	for _, profilePath := range paths {
		obj, err := dbusutil.NewDBusObject(ctx, hermesconst.DBusHermesService, hermesconst.DBusHermesProfileInterface, profilePath)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get dbus object for profile")
		}
		p := Profile{obj}
		s := hermesconst.ProfileStatePending
		if err = p.Property(ctx, hermesconst.ProfilePropertyState, &s); err != nil {
			return nil, errors.Wrap(err, "unable to get profile state")
		}
		for _, desiredState := range desiredStates {
			if s == desiredState {
				profiles = append(profiles, p)
			}
		}
	}
	return profiles, nil
}

// InstalledProfiles reads the eSIM, and returns the installed profiles in the eSIM.
func (e *EUICC) InstalledProfiles(ctx context.Context, shouldNotSwitchSlot bool) ([]Profile, error) {
	if err := e.Call(ctx, hermesconst.EuiccMethodRefreshInstalledProfiles, shouldNotSwitchSlot).Err; err != nil {
		return nil, errors.Wrap(err, "unable to request installed profiles")
	}
	props, err := dbusutil.NewDBusProperties(ctx, e.DBusObject)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get euicc properties")
	}
	profilePaths, err := props.GetObjectPaths(hermesconst.EuiccPropertyInstalledProfiles)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get installed profiles")
	}
	return e.filterProfiles(ctx, profilePaths, []int{hermesconst.ProfileStateEnabled, hermesconst.ProfileStateDisabled})
}

// RefreshSmdxProfiles contacts the SMDX, and returns newly found pending profiles.
func (e *EUICC) RefreshSmdxProfiles(ctx context.Context, rootSMDX string, shouldNotSwitchSlot bool) ([]Profile, error) {
	response := e.Call(ctx, hermesconst.EuiccMethodRefreshSmdxProfiles, rootSMDX, shouldNotSwitchSlot)
	if response.Err != nil {
		return nil, errors.Wrap(response.Err, "unable to refresh SMDX profiles")
	}
	return e.filterProfiles(ctx, response.Body[0].([]dbus.ObjectPath), []int{hermesconst.ProfileStatePending})
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
	euiccPaths, err := GetEUICCPaths(ctx)
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
		response := obj.Call(ctx, hermesconst.EuiccMethodIsTestEuicc)
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

// Eid returns the profile's Eid.
func (e *EUICC) Eid(ctx context.Context) (string, error) {
	return e.getStringProperty(ctx, hermesconst.EuiccPropertyEid)
}

func (e *EUICC) getStringProperty(ctx context.Context, propertyName string) (string, error) {
	props, err := dbusutil.NewDBusProperties(ctx, e.DBusObject)
	if err != nil {
		return "", errors.Wrap(err, "failed to read euicc properties")
	}
	value, err := props.GetString(propertyName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read euicc property %s", propertyName)
	}
	return value, nil
}
