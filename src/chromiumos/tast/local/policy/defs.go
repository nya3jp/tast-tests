// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/json"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
)

// The literal policy types in this file must implement the Policy interface.

// CookiesAllowedForURLs docstring here
///////////////////////////////////////////////////////////////////////////////
// 77. CookiesAllowedForUrls
///////////////////////////////////////////////////////////////////////////////
type CookiesAllowedForURLs struct {
	Stat Status
	Val  []string
}

// Name returns the policy name in string form.
func (p *CookiesAllowedForURLs) Name() string { return "CookiesAllowedForUrls" }

// Field returns the groupname.fieldname for a device policy, or "".
func (p *CookiesAllowedForURLs) Field() string { return "" }

// Scope returns the scope of this policy.
func (p *CookiesAllowedForURLs) Scope() Scope { return UserScope }

// Type returns the type of this policy.
func (p *CookiesAllowedForURLs) Type() Type { return ListType }

// Status returns the status of this policy.
func (p *CookiesAllowedForURLs) Status() Status { return p.Stat }

// UntypedV returns the value of this policy.
func (p *CookiesAllowedForURLs) UntypedV() interface{} { return p.Val }

// Compare returns whether the given JSON value matches this policy.
func (p *CookiesAllowedForURLs) Compare(m json.RawMessage) (bool, error) {
	return listOrderedCompare(m, p.Val)
}

// IncognitoModeAvailability docstring here
///////////////////////////////////////////////////////////////////////////////
// 93. IncognitoModeAvailability
///////////////////////////////////////////////////////////////////////////////
type IncognitoModeAvailability struct {
	Stat Status
	Val  int
}

// Name returns the policy name in string form.
func (p *IncognitoModeAvailability) Name() string { return "IncognitoModeAvailability" }

// Field returns the groupname.fieldname for a device policy, or "".
func (p *IncognitoModeAvailability) Field() string { return "" }

// Scope returns the scope of this policy.
func (p *IncognitoModeAvailability) Scope() Scope { return UserScope }

// Type returns the type of this policy.
func (p *IncognitoModeAvailability) Type() Type { return IntType }

// Status returns the status of this policy.
func (p *IncognitoModeAvailability) Status() Status { return p.Stat }

// UntypedV returns the value of this policy.
func (p *IncognitoModeAvailability) UntypedV() interface{} { return p.Val }

// Compare returns whether the given JSON value matches this policy.
func (p *IncognitoModeAvailability) Compare(m json.RawMessage) (bool, error) {
	return intCompare(m, p.Val)
}

// ExtensionSettings docstring here
///////////////////////////////////////////////////////////////////////////////
// 278. ExtensionSettings
///////////////////////////////////////////////////////////////////////////////
type ExtensionSettings struct {
	Stat Status
	Val  ExtensionSettingsValue
}

// ExtensionSettingsValue docstring here
type ExtensionSettingsValue map[string]*ExtensionSettingsElt

// ExtensionSettingsElt docstring here
type ExtensionSettingsElt struct {
	InstallationMode       string   `json:"installation_mode,omitempty"`
	UpdateURL              string   `json:"update_url,omitempty"`
	BlockedPermssions      []string `json:"blocked_permissions,omitempty"`
	AllowedPermssions      []string `json:"allowed_permissions,omitempty"`
	MinimumVersionRequired string   `json:"minimum_version_required,omitempty"`
	RuntimeBlockedHosts    []string `json:"runtime_blocked_hosts,omitempty"`
	RuntimeAllowedHosts    []string `json:"runtime_allowed_hosts,omitempty"`
	BlockedInstallMessage  string   `json:"blocked_install_message,omitempty"`
}

// Name returns the policy name in string form.
func (p *ExtensionSettings) Name() string { return "ExtensionSettings" }

// Field returns the groupname.fieldname for a device policy, or "".
func (p *ExtensionSettings) Field() string { return "" }

// Scope returns the scope of this policy.
func (p *ExtensionSettings) Scope() Scope { return UserScope }

// Type returns the type of this policy.
func (p *ExtensionSettings) Type() Type { return DictType }

// Status returns the status of this policy.
func (p *ExtensionSettings) Status() Status { return p.Stat }

// UntypedV returns the value of this policy.
func (p *ExtensionSettings) UntypedV() interface{} { return p.Val }

// Compare returns whether the given JSON value matches this policy.
func (p *ExtensionSettings) Compare(m json.RawMessage) (bool, error) {
	var mVal ExtensionSettingsValue
	if err := json.Unmarshal(m, &mVal); err != nil {
		return false, errors.Wrapf(err, "could not read %v as ExtensionSettings", m)
	}
	return cmp.Equal(mVal, p.Val), nil
}

// AllowDinosaurEasterEgg docstring here
///////////////////////////////////////////////////////////////////////////////
// 309. AllowDinosaurEasterEgg
///////////////////////////////////////////////////////////////////////////////
type AllowDinosaurEasterEgg struct {
	Stat Status
	Val  bool
}

// Name returns the policy name in string form.
func (p *AllowDinosaurEasterEgg) Name() string { return "AllowDinosaurEasterEgg" }

// Field returns the groupname.fieldname for a device policy, or "".
func (p *AllowDinosaurEasterEgg) Field() string { return "" }

// Scope returns the scope of this policy.
func (p *AllowDinosaurEasterEgg) Scope() Scope { return UserScope }

// Type returns the type of this policy.
func (p *AllowDinosaurEasterEgg) Type() Type { return BoolType }

// Status returns the status of this policy.
func (p *AllowDinosaurEasterEgg) Status() Status { return p.Stat }

// UntypedV returns the value of this policy.
func (p *AllowDinosaurEasterEgg) UntypedV() interface{} { return p.Val }

// Compare returns whether the given JSON value matches this policy.
func (p *AllowDinosaurEasterEgg) Compare(m json.RawMessage) (bool, error) {
	return boolCompare(m, p.Val)
}

// DeviceAutoUpdateTimeRestrictions docstring here
///////////////////////////////////////////////////////////////////////////////
// 453. DeviceAutoUpdateTimeRestrictions
///////////////////////////////////////////////////////////////////////////////
type DeviceAutoUpdateTimeRestrictions struct {
	Stat Status
	Val  []*DeviceAutoUpdateTimeRestrictionsValue
}

// DeviceAutoUpdateTimeRestrictionsValue docstring here
type DeviceAutoUpdateTimeRestrictionsValue struct {
	Start DeviceAutoUpdateTimeRestrictionsDayMinuteHour `json:"start"`
	End   DeviceAutoUpdateTimeRestrictionsDayMinuteHour `json:"end"`
}

// DeviceAutoUpdateTimeRestrictionsDayMinuteHour docstring here
type DeviceAutoUpdateTimeRestrictionsDayMinuteHour struct {
	DayOfWeek string `json:"day_of_week"`
	Minutes   int    `json:"minutes"`
	Hours     int    `json:"hours"`
}

// AddDeviceAutoUpdateTimeRestriction docstring here
func (p *DeviceAutoUpdateTimeRestrictions) AddDeviceAutoUpdateTimeRestriction(
	sd string, sm, sh int, ed string, em, eh int) {
	newDAUTR := DeviceAutoUpdateTimeRestrictionsValue{
		Start: DeviceAutoUpdateTimeRestrictionsDayMinuteHour{
			DayOfWeek: sd, Minutes: sm, Hours: sh},
		End: DeviceAutoUpdateTimeRestrictionsDayMinuteHour{
			DayOfWeek: ed, Minutes: em, Hours: eh},
	}
	p.Val = append(p.Val, &newDAUTR)
}

// Name returns the policy name in string form.
func (p *DeviceAutoUpdateTimeRestrictions) Name() string { return "DeviceAutoUpdateTimeRestrictions" }

// Field returns the groupname.fieldname for a device policy, or "".
func (p *DeviceAutoUpdateTimeRestrictions) Field() string {
	return "auto_update_settings.disallowed_time_intervals"
}

// Scope returns the scope of this policy.
func (p *DeviceAutoUpdateTimeRestrictions) Scope() Scope { return UserScope }

// Type returns the type of this policy.
func (p *DeviceAutoUpdateTimeRestrictions) Type() Type { return DictType }

// Status returns the status of this policy.
func (p *DeviceAutoUpdateTimeRestrictions) Status() Status { return p.Stat }

// UntypedV returns the value of this policy.
func (p *DeviceAutoUpdateTimeRestrictions) UntypedV() interface{} { return p.Val }

// Compare returns whether the given JSON value matches this policy.
func (p *DeviceAutoUpdateTimeRestrictions) Compare(m json.RawMessage) (bool, error) {
	var mVal []*DeviceAutoUpdateTimeRestrictionsValue
	if err := json.Unmarshal(m, &mVal); err != nil {
		return false, errors.Wrapf(err,
			"could not read %v as DeviceAutoUpdateTimeRestrictions", m)
	}
	if len(mVal) != len(p.Val) {
		return false, nil
	}
	for i := range mVal {
		if !cmp.Equal(mVal[i], p.Val[i]) {
			return false, nil
		}
	}
	return true, nil
}
