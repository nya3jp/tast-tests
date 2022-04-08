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
	DBusModemmanagerBearerInterface      = "org.freedesktop.ModemManager1.Bearer"
	DBusModemmanagerModemInterface       = "org.freedesktop.ModemManager1.Modem"
	DBusModemmanager3gppModemInterface   = "org.freedesktop.ModemManager1.Modem.Modem3gpp"
	DBusModemmanagerSimpleModemInterface = "org.freedesktop.ModemManager1.Modem.Simple"
	DBusModemmanagerSARInterface         = "org.freedesktop.ModemManager1.Modem.Sar"
	DBusModemmanagerSimInterface         = "org.freedesktop.ModemManager1.Sim"
)
