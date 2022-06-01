// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hermesconst defines the constants for Hermes
// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/hermes/README.md
// This package is defined under common/ as they might be used in both
// local and remote tests.
package hermesconst

// Choose test type
const (
	HermesOnly  = "hermes_only"
	HermesAndMM = "hermes_and_mm"
)

// Hermes D-Bus constants.
const (
	DBusHermesManagerPath      = "/org/chromium/Hermes/Manager"
	DBusHermesService          = "org.chromium.Hermes"
	DBusHermesManagerInterface = "org.chromium.Hermes.Manager"
	DBusHermesEuiccInterface   = "org.chromium.Hermes.Euicc"
	DBusHermesProfileInterface = "org.chromium.Hermes.Profile"
)

// Hermes.Euicc properties
const (
	ManagerPropertyAvailableEuiccs = "AvailableEuiccs"
)

// Hermes.Euicc methods
const (
	EuiccMethodRefreshInstalledProfiles         = "RefreshInstalledProfiles"
	EuiccMethodInstallPendingProfile            = "InstallPendingProfile"
	EuiccMethodInstallProfileFromActivationCode = "InstallProfileFromActivationCode"
	EuiccMethodUninstallProfile                 = "UninstallProfile"
	EuiccMethodResetMemory                      = "ResetMemory"
)

// Hermes.Euicc properties
const (
	EuiccPropertyProfileRefreshedAtLeastOnce = "ProfilesRefreshedAtLeastOnce"
)

// Hermes.Profile methods
const (
	ProfileMethodEnable  = "Enable"
	ProfileMethodDisable = "Disable"
	ProfileMethodRename  = "Rename"
)

// Hermes.Profile properties
const (
	ProfilePropertyState    = "State"
	ProfilePropertyClass    = "ProfileClass"
	ProfilePropertyIccid    = "Iccid"
	ProfilePropertyNickname = "Nickname"
)

// States that a Hermes profile can be in
const (
	ProfileStatePending  = 0
	ProfileStateDisabled = 1
	ProfileStateEnabled  = 2
)

// Types of eSIM profiles
const (
	ProfileClassTest         = 0
	ProfileClassProvisioning = 1
	ProfileClassOperational  = 2
)

// Google SMDS Server address, passing an empty |root_smds| will use default lpa.ds.gsma.com.
const (
	RootSmdsAddress = "prod.smds.rsp.goog"
)
