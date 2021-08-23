// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package modemmanager provides D-Bus wrappers and utilities for modemmanager.
package modemmanager

// ModemManager1 DBus constants
const (
	DBusModemmanagerPath                 = "/org/freedesktop/ModemManager1"
	DBusModemmanagerService              = "org.freedesktop.ModemManager1"
	DBusModemmanagerInterface            = "org.freedesktop.ModemManager1"
	DBusModemmanagerModemInterface       = "org.freedesktop.ModemManager1.Modem"
	DBusModemmanagerSimpleModemInterface = "org.freedesktop.ModemManager.Modem.Simple"
	DBusModemmanagerSimInterface         = "org.freedesktop.ModemManager1.Sim"
)
