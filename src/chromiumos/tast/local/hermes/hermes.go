// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hermes provides D-Bus wrappers and utilities for Hermes.
// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/hermes/README.md
package hermes

// Hermes D-Bus constants.
const (
	DBusHermesManagerPath      = "/org/chromium/Hermes/Manager"
	DBusHermesService          = "org.chromium.Hermes"
	DBusHermesManagerInterface = "org.chromium.Hermes.Manager"
	DBusHermesEuiccInterface   = "org.chromium.Hermes.Euicc"
	DBusHermesProfileInterface = "org.chromium.Hermes.Profile"
)
