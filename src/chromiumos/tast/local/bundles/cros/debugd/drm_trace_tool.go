// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	dbusName      = "org.chromium.debugd"
	dbusPath      = "/org/chromium/debugd"
	dbusInterface = "org.chromium.debugd"
)

// Must match the DRMTraceSize enum defined in org.chromium.debugd.xml.
const (
	drmTraceSizeDefault uint32 = 0
	drmTraceSizeDebug   uint32 = 1
)

// Must match the DRMTraceCategories flags defined in org.chromium.debugd.xml.
const (
	drmTraceCategoryCore   = 0x001
	drmTraceCategoryDriver = 0x002
	drmTraceCategoryKMS    = 0x004
	drmTraceCategoryPrime  = 0x008
	drmTraceCategoryAtomic = 0x010
	drmTraceCategoryVBL    = 0x020
	drmTraceCategoryState  = 0x040
	drmTraceCategoryLease  = 0x080
	drmTraceCategoryDP     = 0x100
	drmTraceCategoryDRMRes = 0x200
)

const (
	bufferSizePath    = "/sys/kernel/debug/tracing/instances/drm/buffer_size_kb"
	traceContentsPath = "/sys/kernel/debug/tracing/instances/drm/trace"
	traceMaskPath     = "/sys/module/drm/parameters/trace"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DRMTraceTool,
		Desc: "Tests D-Bus methods related to DRMTraceTool",
		Contacts: []string{
			"ddavenport@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"drm_trace"},
	})
}

// DRMTraceTool tests D-bus methods related to debugd's DRMTraceTool.
func DRMTraceTool(ctx context.Context, s *testing.State) {
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	s.Log("Verify DRMTraceAnnotateLog method")
	// Create a log annotation that will not already be in the log by chance, or from a previous run of this test.
	log := fmt.Sprintf("annotate-%s", time.Now().Unix())
	err = testAnnotate(ctx, obj, log)
	if err != nil {
		s.Error("Failed to verify DRMTraceAnnotateLog: ", err)
	}

	s.Log("Verify DRMTraceSetCategories method")
	const testCategories uint32 = drmTraceCategoryCore | drmTraceCategoryKMS
	err = testSetCategories(ctx, obj, testCategories, testCategories)
	if err != nil {
		s.Error("Failed to verify DRMTraceSetCategories: ", err)
	}

	const expectedDefaultCategories uint32 = drmTraceCategoryDriver | drmTraceCategoryKMS | drmTraceCategoryDP
	err = testSetCategories(ctx, obj, uint32(0), expectedDefaultCategories)
	if err != nil {
		s.Error("Failed to verify DRMTraceSetCategories: ", err)
	}

	s.Log("Verify DRMTraceSetSize method")
	err = testSetSize(ctx, obj)
	if err != nil {
		s.Error("Failed to verify DRMTraceSetSize: ", err)
	}
}

// testAnnotate verifies the DRMTraceAnnotateLog D-Bus method.
func testAnnotate(ctx context.Context, obj dbus.BusObject, log string) error {
	if err := obj.CallWithContext(ctx, dbusInterface+".DRMTraceAnnotateLog", 0, log).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceAnnotateLog")
	}

	// Check that this ended up in the trace file.
	contents, err := readFileToString(traceContentsPath)
	if err != nil {
		return errors.Wrap(err, "failed to read trace log")
	}

	if !strings.Contains(contents, log) {
		return errors.Errorf("failed to find log %s in drm trace", log)
	}
	return nil
}

// testSetCategories verifies the DRMTraceSetCategories D-Bus method.
func testSetCategories(ctx context.Context, obj dbus.BusObject, category, expected uint32) error {
	if err := obj.CallWithContext(ctx, dbusInterface+".DRMTraceSetCategories", 0, category).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetCategories")
	}

	// Check that this ended up in the mask file.
	mask, err := readFileToInt(traceMaskPath)
	if err != nil {
		return errors.Wrap(err, "failed to read trace mask")
	}

	if mask != int(expected) {
		return errors.Errorf("unexpected value for trace mask. Got %d and expected %d", mask, expected)
	}
	return nil
}

// testSetSize verifies the DRMTraceSetSize D-Bus method.
func testSetSize(ctx context.Context, obj dbus.BusObject) error {
	// Ensure it is set to default.
	if err := obj.CallWithContext(ctx, dbusInterface+".DRMTraceSetSize", 0, drmTraceSizeDefault).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetSize")
	}

	// Check the size.
	defaultSize, err := readFileToInt(bufferSizePath)
	if err != nil {
		return errors.Wrap(err, "failed to read trace buffer size")
	}

	// Set to debug.
	if err := obj.CallWithContext(ctx, dbusInterface+".DRMTraceSetSize", 0, drmTraceSizeDebug).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetSize")
	}

	debugSize, err := readFileToInt(bufferSizePath)
	if err != nil {
		return errors.Wrap(err, "failed to read trace buffer size")
	}

	if debugSize <= defaultSize {
		return errors.Errorf("expected debug size (%d) to be greater than default size (%d)", debugSize, defaultSize)
	}
	return nil
}

// readFileToString opens the file at |path|, reads its contents, and returns the contents as a string.
// The contents are trimmed of leading and trailing whitespace.
func readFileToString(path string) (string, error) {
	var contents string
	bytes, err := ioutil.ReadFile(path)
	if err == nil {
		contents = strings.TrimSpace(string(bytes))
	}
	return contents, err
}

// readFileToInt opens the file at |path|, reads its contents, and returns the contents as an int.
func readFileToInt(path string) (int, error) {
	var size int
	sizeStr, err := readFileToString(path)
	if err == nil {
		size, err = strconv.Atoi(sizeStr)
	}
	return size, err
}
