// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package debugd interacts with debugd D-Bus service.
package debugd

import (
	"context"
	"fmt"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName            = "org.chromium.debugd"
	dbusPath            = "/org/chromium/debugd"
	dbusInterface       = "org.chromium.debugd"
	crashSenderTestMode = "CrashSenderTestMode"
)

// Scheduler describes a scheduler mode that can be applied via SetSchedulerConfiguration.
type Scheduler string

const (
	// Conservative scheduler favors stability.
	Conservative Scheduler = "conservative"
	// Performance scheduler favors speed.
	Performance = "performance"
)

// CUPSResult is a status code for the CUPS related debugd D-Bus methods.
// Values are from platform2/system_api/dbus/debugd/dbus-constants.h
type CUPSResult int32

const (
	// CUPSSuccess indicates the operation succeeded.
	CUPSSuccess CUPSResult = 0

	// CUPSFatal indicates the operation failed for an unknown reason.
	CUPSFatal CUPSResult = 1

	// CUPSInvalidPPD indicates the operation failed because the given PPD is invalid.
	CUPSInvalidPPD CUPSResult = 2

	// CUPSLPAdminFailure indicates the operation failed because lpadmin command failed.
	CUPSLPAdminFailure CUPSResult = 3

	// CUPSAutoconfFailure indicates the operation failed due to autoconf failures.
	CUPSAutoconfFailure CUPSResult = 4

	// CUPSBadURI indicates that the operation failed because debugd
	// rejected the printer URI.
	CUPSBadURI CUPSResult = 5
)

func (r CUPSResult) String() string {
	switch r {
	case CUPSSuccess:
		return fmt.Sprintf("CUPSSuccess(%d)", r)
	case CUPSFatal:
		return fmt.Sprintf("CUPSFatal(%d)", r)
	case CUPSInvalidPPD:
		return fmt.Sprintf("CUPSInvalidPPD(%d)", r)
	case CUPSLPAdminFailure:
		return fmt.Sprintf("CUPSLPAdminFailure(%d)", r)
	case CUPSAutoconfFailure:
		return fmt.Sprintf("CUPSAutoconfFailure(%d)", r)
	case CUPSBadURI:
		return fmt.Sprintf("CUPSBadURI(%d)", r)
	default:
		return fmt.Sprintf("Unknown(%d)", r)
	}
}

// Debugd is used to interact with the debugd process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/debugd/dbus_bindings/org.chromium.debugd.xml
type Debugd struct {
	obj dbus.BusObject
}

// New connects to debugd via D-Bus and returns a Debugd object.
func New(ctx context.Context) (*Debugd, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &Debugd{obj}, nil
}

// CupsAddAutoConfiguredPrinter calls debugd.CupsAddAutoConfiguredPrinter D-Bus method.
func (d *Debugd) CupsAddAutoConfiguredPrinter(ctx context.Context, name, uri string) (CUPSResult, error) {
	c := d.call(ctx, "CupsAddAutoConfiguredPrinter", name, uri)
	var status int32
	if err := c.Store(&status); err != nil {
		return 0, err
	}
	return CUPSResult(status), nil
}

// CupsAddManuallyConfiguredPrinter calls debugd.CupsAddManuallyConfiguredPrinter D-Bus method.
func (d *Debugd) CupsAddManuallyConfiguredPrinter(ctx context.Context, name, uri string, ppdContents []byte) (CUPSResult, error) {
	c := d.call(ctx, "CupsAddManuallyConfiguredPrinter", name, uri, ppdContents)
	var status int32
	if err := c.Store(&status); err != nil {
		return 0, err
	}
	return CUPSResult(status), nil
}

// SetSchedulerConfiguration calls debugd's SetSchedulerConfiguration D-Bus method.
func (d *Debugd) SetSchedulerConfiguration(ctx context.Context, param Scheduler) (err error) {
	result := false
	if err := d.call(ctx, "SetSchedulerConfiguration", string(param)).Store(&result); err != nil {
		return err
	} else if !result {
		return errors.New("SetSchedulerConfiguration returned false")
	}
	return nil
}

// SetCrashSenderTestMode sets debugd's CrashSenderTestMode property. If this is
// set to true, the crash_sender invoked from debugd will just touch the "test
// successful" file instead of uploading crashes.
func (d *Debugd) SetCrashSenderTestMode(ctx context.Context, testMode bool) (err error) {
	return d.obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Set", 0, dbusInterface, crashSenderTestMode, testMode).Err
}

// call is thin wrapper of CallWithContext for convenience.
func (d *Debugd) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return d.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
