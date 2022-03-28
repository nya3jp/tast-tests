// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hermesconst defines the constants for Hermes
// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/hermes/README.md
// This package is defined under common/ as they might be used in both
// local and remote tests.
package hermesconst

// Path to gsma test certs used to communicate with stork
const (
	GsmaTestCertPath = "/usr/share/hermes-ca-certificates/test/gsma-ci.pem"
)

// Hermes.Euicc properties
const (
	ManagerPropertyAvailableEuiccs = "AvailableEuiccs"
)

// Hermes.Euicc methods
const (
	EuiccMethodRefreshInstalledProfiles         = "RefreshInstalledProfiles"
	EuiccMethodInstallProfileFromActivationCode = "InstallProfileFromActivationCode"
	EuiccMethodUninstallProfile                 = "UninstallProfile"
	EuiccMethodResetMemory                      = "ResetMemory"
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
