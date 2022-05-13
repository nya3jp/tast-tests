// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
)

// DBusError strings from https://www.freedesktop.org/software/systemd/man/sd-bus-errors.html
const (
	DBusErrorFailed                           = "org.freedesktop.DBus.Error.Failed"
	DBusErrorNoMemory                         = "org.freedesktop.DBus.Error.NoMemory"
	DBusErrorServiceUnknown                   = "org.freedesktop.DBus.Error.ServiceUnknown"
	DBusErrorNameHasNoOwner                   = "org.freedesktop.DBus.Error.NameHasNoOwner"
	DBusErrorNoReply                          = "org.freedesktop.DBus.Error.NoReply"
	DBusErrorIOError                          = "org.freedesktop.DBus.Error.IOError"
	DBusErrorBadAddress                       = "org.freedesktop.DBus.Error.BadAddress"
	DBusErrorNotSupported                     = "org.freedesktop.DBus.Error.NotSupported"
	DBusErrorLimitsExceeded                   = "org.freedesktop.DBus.Error.LimitsExceeded"
	DBusErrorAccessDenied                     = "org.freedesktop.DBus.Error.AccessDenied"
	DBusErrorAuthFailed                       = "org.freedesktop.DBus.Error.AuthFailed"
	DBusErrorNoServer                         = "org.freedesktop.DBus.Error.NoServer"
	DBusErrorTimeout                          = "org.freedesktop.DBus.Error.Timeout"
	DBusErrorNoNetwork                        = "org.freedesktop.DBus.Error.NoNetwork"
	DBusErrorAddressInUse                     = "org.freedesktop.DBus.Error.AddressInUse"
	DBusErrorDisconnected                     = "org.freedesktop.DBus.Error.Disconnected"
	DBusErrorInvalidArgs                      = "org.freedesktop.DBus.Error.InvalidArgs"
	DBusErrorFileNotFound                     = "org.freedesktop.DBus.Error.FileNotFound"
	DBusErrorFileExists                       = "org.freedesktop.DBus.Error.FileExists"
	DBusErrorUnknownMethod                    = "org.freedesktop.DBus.Error.UnknownMethod"
	DBusErrorUnknownObject                    = "org.freedesktop.DBus.Error.UnknownObject"
	DBusErrorUnknownInterface                 = "org.freedesktop.DBus.Error.UnknownInterface"
	DBusErrorUnknownProperty                  = "org.freedesktop.DBus.Error.UnknownProperty"
	DBusErrorPropertyReadOnly                 = "org.freedesktop.DBus.Error.PropertyReadOnly"
	DBusErrorUnixProcessIDUnknown             = "org.freedesktop.DBus.Error.UnixProcessIdUnknown"
	DBusErrorInvalidSignature                 = "org.freedesktop.DBus.Error.InvalidSignature"
	DBusErrorInconsistentMessage              = "org.freedesktop.DBus.Error.InconsistentMessage"
	DBusErrorMatchRuleNotFound                = "org.freedesktop.DBus.Error.MatchRuleNotFound"
	DBusErrorMatchRuleInvalid                 = "org.freedesktop.DBus.Error.MatchRuleInvalid"
	DBusErrorInteractiveAuthorizationRequired = "org.freedesktop.DBus.Error.InteractiveAuthorizationRequired"
)

// IsDBusError checks if err is a wrapped dbus error with given error name.
func IsDBusError(err error, name string) bool {
	var dbusErr dbus.Error
	if !errors.As(err, &dbusErr) {
		return false
	}
	return dbusErr.Name == name
}
