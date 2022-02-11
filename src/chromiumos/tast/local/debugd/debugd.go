// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package debugd interacts with debugd D-Bus service.
package debugd

import (
	"context"
	"fmt"
	"os"

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

	// CUPSIOError indicates that the operation failed because of I/O error.
	CUPSIOError CUPSResult = 6

	// CUPSMemoryAllocError indicates that the operation failed because of
	// memory allocation error.
	CUPSMemoryAllocError CUPSResult = 7

	// CUPSPrinterUnreachable indicates that the printer did not respond.
	CUPSPrinterUnreachable CUPSResult = 8

	// CUPSPrinterWrongResponse indicates that the printer sent
	// an unexpected response.
	CUPSPrinterWrongResponse CUPSResult = 9

	// CUPSPrinterNotAutoconf indicates that the operation failed because
	// the printer is not autoconfigurable as it supposed to be.
	CUPSPrinterNotAutoconf CUPSResult = 10
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
	case CUPSIOError:
		return fmt.Sprintf("CUPSIOError(%d)", r)
	case CUPSMemoryAllocError:
		return fmt.Sprintf("CUPSMemoryAllocError(%d)", r)
	case CUPSPrinterUnreachable:
		return fmt.Sprintf("CUPSPrinterUnreachable(%d)", r)
	case CUPSPrinterWrongResponse:
		return fmt.Sprintf("CUPSPrinterWrongResponse(%d)", r)
	case CUPSPrinterNotAutoconf:
		return fmt.Sprintf("CUPSPrinterNotAutoconf(%d)", r)
	default:
		return fmt.Sprintf("Unknown(%d)", r)
	}
}

// DRMTraceSize is an enumeration used as an argument to the DRMTraceSetSize method.
type DRMTraceSize uint32

// This must match the DRMTraceSize enum defined in org.chromium.debugd.xml.
const (
	DRMTraceSizeDefault DRMTraceSize = 0
	DRMTraceSizeDebug                = 1
)

// DRMTraceSnapshotType is an enumeration used as an argument to the DRMTraceSnapshot method.
type DRMTraceSnapshotType uint32

// This must match the DRMTraceSnapshotType enum defined in org.chromium.debugd.xml.
const (
	DRMTraceSnapshotTypeTrace    DRMTraceSnapshotType = 0
	DRMTraceSnapshotTypeModetest DRMTraceSnapshotType = 1
)

// DRMTraceCategories is a bitmask used as an argument to the DRMTraceSetCategories method.
type DRMTraceCategories uint32

// This must match the DRMTraceCategories flags defined in org.chromium.debugd.xml.
const (
	DRMTraceCategoryCore   DRMTraceCategories = 0x001
	DRMTraceCategoryDriver                    = 0x002
	DRMTraceCategoryKMS                       = 0x004
	DRMTraceCategoryPrime                     = 0x008
	DRMTraceCategoryAtomic                    = 0x010
	DRMTraceCategoryVBL                       = 0x020
	DRMTraceCategoryState                     = 0x040
	DRMTraceCategoryLease                     = 0x080
	DRMTraceCategoryDP                        = 0x100
	DRMTraceCategoryDRMRes                    = 0x200
)

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

// CupsRemovePrinter calls debugd.CupsRemovePrinter D-Bus method.
func (d *Debugd) CupsRemovePrinter(ctx context.Context, name string) error {
	c := d.call(ctx, "CupsRemovePrinter", name)
	result := false
	if err := c.Store(&result); err != nil {
		return err
	}
	if !result {
		return errors.New("CupsRemovePrinter returned false")
	}
	return nil
}

// SetSchedulerConfiguration calls debugd's SetSchedulerConfigurationV2 D-Bus method.
func (d *Debugd) SetSchedulerConfiguration(ctx context.Context, param Scheduler) (err error) {
	result := false
	var numCoresDisabled uint32
	// TODO(abhishekbh): Support calling the D-Bus method with lock_policy=true, and add test cases to debugd/core_scheduler.go.
	if err := d.call(ctx, "SetSchedulerConfigurationV2", string(param), false).Store(&result, &numCoresDisabled); err != nil {
		return err
	} else if !result {
		return errors.New("SetSchedulerConfiguration returned false")
	}
	return nil
}

// PacketCaptureStart calls debugd's PacketCaptureStart D-Bus method.
func (d *Debugd) PacketCaptureStart(ctx context.Context, output, stat *os.File, options map[string]dbus.Variant) (h string, err error) {
	// The handle of the packet capture process that is started by the D-Bus call.
	var handle string
	c := d.call(ctx, "PacketCaptureStart", dbus.UnixFD(stat.Fd()), dbus.UnixFD(output.Fd()), options)
	if c.Store(&handle) != nil {
		return handle, errors.Wrap(c.Err, "Packet capture start D-Bus call failed")
	}
	return handle, nil
}

// PacketCaptureStop calls debugd's PacketCaptureStop D-Bus method.
func (d *Debugd) PacketCaptureStop(ctx context.Context, handle string) (err error) {
	if err := d.call(ctx, "PacketCaptureStop", handle).Err; err != nil {
		return errors.Wrap(err, "Packet capture stop D-Bus call failed")
	}
	return nil
}

// SetCrashSenderTestMode sets debugd's CrashSenderTestMode property. If this is
// set to true, the crash_sender invoked from debugd will just touch the "test
// successful" file instead of uploading crashes.
func (d *Debugd) SetCrashSenderTestMode(ctx context.Context, testMode bool) (err error) {
	return d.obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Set", 0, dbusInterface, crashSenderTestMode, testMode).Err
}

// DRMTraceAnnotateLog calls debugd's DRMTraceAnnotateLog D-Bus method.
func (d *Debugd) DRMTraceAnnotateLog(ctx context.Context, log string) (err error) {
	if err := d.call(ctx, "DRMTraceAnnotateLog", log).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceAnnotateLog")
	}
	return nil
}

// DRMTraceSetCategories calls debugd's DRMTraceSetCategories D-Bus method.
func (d *Debugd) DRMTraceSetCategories(ctx context.Context, categories DRMTraceCategories) (err error) {
	if err := d.call(ctx, "DRMTraceSetCategories", uint32(categories)).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceAnnotateLog")
	}
	return nil
}

// DRMTraceSetSize calls debugd's DRMTraceSetSize D-Bus method.
func (d *Debugd) DRMTraceSetSize(ctx context.Context, size DRMTraceSize) (err error) {
	if err := d.call(ctx, "DRMTraceSetSize", uint32(size)).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetSize")
	}
	return nil
}

// DRMTraceSnapshot calls debugd's DRMTraceSnapshot D-Bus method.
func (d *Debugd) DRMTraceSnapshot(ctx context.Context, snapshotType DRMTraceSnapshotType) (err error) {
	if err := d.call(ctx, "DRMTraceSnapshot", uint32(snapshotType)).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSnapshot")
	}
	return nil
}

// GetPerfOutputV2 calls debugd's GetPerfOutputV2 D-Bus method.
func (d *Debugd) GetPerfOutputV2(ctx context.Context, quipperArgs []string, disableCPUIdle bool) ([]byte, []byte, int, error) {
	status := 0
	var perfData, perfStat []byte
	c := d.call(ctx, "GetPerfOutputV2", quipperArgs, disableCPUIdle)
	if c.Err != nil {
		return nil, nil, status, errors.Wrap(c.Err, "failed to call GetPerfOutputV2")
	}
	err := c.Store(&status, &perfData, &perfStat)
	return perfData, perfStat, status, err
}

// call is thin wrapper of CallWithContext for convenience.
func (d *Debugd) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return d.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
