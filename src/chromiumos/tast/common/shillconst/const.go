// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shillconst defines the constants of shill service.
// This is defined under common/ as they might be used in both
// local and remote tests.
package shillconst

import "github.com/godbus/dbus"

// ServiceKeyMgmtIEEE8021X is a value of EAPKeyMgmt.
const ServiceKeyMgmtIEEE8021X = "IEEE8021X"

const defaultStorageDir = "/var/cache/shill/"

const (
	// DefaultProfileName is the name of default profile.
	DefaultProfileName = "default"
	// DefaultProfileObjectPath is the dbus object path of default profile.
	DefaultProfileObjectPath dbus.ObjectPath = "/profile/" + DefaultProfileName
	// DefaultProfilePath is the path of default profile.
	DefaultProfilePath = defaultStorageDir + DefaultProfileName + ".profile"
)

// Profile entry property names.
const (
	ProfileEntryPropertyName = "Name"
	ProfileEntryPropertyType = "Type"
)
