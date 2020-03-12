// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import "chromiumos/tast/errors"

// Type values defined in dbus-constants.h
// The values are used both for Service type and Technology type.
const (
	TypeEthernet = "ethernet"
	TypeWifi     = "wifi"
	TypeCellular = "cellular"
	TypeVPN      = "vpn"
	TypePPPoE    = "pppoe"
)

// ErrInvalidPath is returned when the dbus method call failed due to
// invalid object path. This usually means the device/service/... is
// removed.
var ErrInvalidPath = errors.New("invalid object path")
