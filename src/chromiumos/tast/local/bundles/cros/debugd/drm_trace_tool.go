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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
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
	dbgd, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd D-Bus service: ", err)
	}

	s.Log("Verify DRMTraceAnnotateLog method")
	// Create a log annotation that will not already be in the log by chance, or from a previous run of this test.
	log := fmt.Sprintf("annotate-%d", time.Now().Unix())
	if err := testAnnotate(ctx, dbgd, log); err != nil {
		s.Error("Failed to verify DRMTraceAnnotateLog: ", err)
	}

	s.Log("Verify DRMTraceSetCategories method")
	const testCategories = debugd.DRMTraceCategoryCore | debugd.DRMTraceCategoryKMS
	if err := testSetCategories(ctx, dbgd, testCategories, testCategories); err != nil {
		s.Error("Failed to verify DRMTraceSetCategories: ", err)
	}

	const (
		defaultCategories         = 0
		expectedDefaultCategories = debugd.DRMTraceCategoryDriver | debugd.DRMTraceCategoryKMS | debugd.DRMTraceCategoryDP
	)
	if err := testSetCategories(ctx, dbgd, defaultCategories, expectedDefaultCategories); err != nil {
		s.Error("Failed to verify DRMTraceSetCategories: ", err)
	}

	s.Log("Verify DRMTraceSetSize method")
	if err := testSetSize(ctx, dbgd); err != nil {
		s.Error("Failed to verify DRMTraceSetSize: ", err)
	}
}

// testAnnotate verifies the DRMTraceAnnotateLog D-Bus method.
func testAnnotate(ctx context.Context, d *debugd.Debugd, log string) error {
	if err := d.DRMTraceAnnotateLog(ctx, log); err != nil {
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
func testSetCategories(ctx context.Context, d *debugd.Debugd, categories, expected debugd.DRMTraceCategories) error {
	if err := d.DRMTraceSetCategories(ctx, categories); err != nil {
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
func testSetSize(ctx context.Context, d *debugd.Debugd) error {
	// Ensure it is set to default.
	if err := d.DRMTraceSetSize(ctx, debugd.DRMTraceSizeDefault); err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetSize")
	}

	// Check the size.
	defaultSize, err := readFileToInt(bufferSizePath)
	if err != nil {
		return errors.Wrap(err, "failed to read trace buffer size")
	}

	// Set to debug.
	if err := d.DRMTraceSetSize(ctx, debugd.DRMTraceSizeDebug); err != nil {
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
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}

// readFileToInt opens the file at |path|, reads its contents, and returns the contents as an int.
func readFileToInt(path string) (int, error) {
	sizeStr, err := readFileToString(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(sizeStr)
}
