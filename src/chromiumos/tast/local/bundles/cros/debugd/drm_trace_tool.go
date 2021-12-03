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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
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
		SoftwareDeps: []string{"chrome", "drm_trace"},
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

	s.Log("Verify DRMTraceTool parameters are reset correctly")
	sm := func() *session.SessionManager {
		// Set up the test environment. Should be done quickly.
		const setupTimeout = 30 * time.Second
		setupCtx, cancel := context.WithTimeout(ctx, setupTimeout)
		defer cancel()

		// Ensures login screen.
		if err := upstart.RestartJob(setupCtx, "ui"); err != nil {
			s.Fatal("Chrome logout failed: ", err)
		}

		sm, err := session.NewSessionManager(setupCtx)
		if err != nil {
			s.Fatal("Failed to connect session_manager: ", err)
		}
		return sm
	}()
	if err := testLogin(ctx, dbgd, sm); err != nil {
		s.Error("Failed to verify DRMTraceTool parameter reset on login: ", err)
	}
	if err := testLogout(ctx, dbgd, sm); err != nil {
		s.Error("Failed to verify DRMTraceTool parameter reset on logout: ", err)
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

// testLogin verifies that drm_trace related configuration is reset correctly on login.
func testLogin(ctx context.Context, d *debugd.Debugd, sm *session.SessionManager) error {
	// Record the current (default) values for the mask and size.
	defaultMask, defaultSize, err := readTraceMaskAndSize()
	if err != nil {
		return err
	}

	// Before login, set the mask and buffer size to some non-default values.
	const testCategories debugd.DRMTraceCategories = debugd.DRMTraceCategoryCore | debugd.DRMTraceCategoryKMS
	if err := setTraceCategoriesAndSize(ctx, d, testCategories, debugd.DRMTraceSizeDebug); err != nil {
		return err
	}

	// Log in to Chrome, and wait for login to be complete.
	if err := chromeLogin(ctx, sm); err != nil {
		return err
	}

	// Get the mask and size after login is complete.
	afterLoginMask, afterLoginSize, err := readTraceMaskAndSize()
	if err != nil {
		return err
	}

	// Verify that the mask and buffer are set to default after logging in.
	if afterLoginSize != defaultSize {
		return errors.Errorf("expected buffer size (%d) to be reset to default size (%d)", afterLoginSize, defaultSize)
	}

	if afterLoginMask != defaultMask {
		return errors.Errorf("expected trace mask (%d) to be reset to default mask (%d)", afterLoginMask, defaultSize)
	}
	return nil
}

// testLogout verifies that drm_trace related configuration is reset correctly on logout.
func testLogout(ctx context.Context, d *debugd.Debugd, sm *session.SessionManager) error {
	// Log in to Chrome, and wait for login to be complete.
	if err := chromeLogin(ctx, sm); err != nil {
		return err
	}

	// Record the current (default) values for the mask and size.
	defaultMask, defaultSize, err := readTraceMaskAndSize()
	if err != nil {
		return err
	}

	// Before logout, set the mask and buffer size to some non-default values.
	const testCategories debugd.DRMTraceCategories = debugd.DRMTraceCategoryCore | debugd.DRMTraceCategoryKMS
	if err := setTraceCategoriesAndSize(ctx, d, testCategories, debugd.DRMTraceSizeDebug); err != nil {
		return err
	}

	// Emulate a logout by restarting "ui".
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to log out")
	}

	// Get the mask and size after login is complete.
	afterLogoutMask, afterLogoutSize, err := readTraceMaskAndSize()
	if err != nil {
		return err
	}

	// Verify that the mask and buffer are set to default.
	if afterLogoutSize != defaultSize {
		return errors.Errorf("expected buffer size (%d) to be reset to default size (%d)", afterLogoutSize, defaultSize)
	}

	if afterLogoutMask != defaultMask {
		return errors.Errorf("expected trace mask (%d) to be reset to default mask (%d)", afterLogoutMask, defaultSize)
	}
	return nil
}

// chromeLogin will restart the UI and log in to Chrome, waiting for the "started" signal before returning.
func chromeLogin(ctx context.Context, sm *session.SessionManager) error {
	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		return errors.Wrap(err, "failed to watch for D-Bus signals")
	}
	defer sw.Close(ctx)

	cr, err := chrome.New(ctx)
	if err != nil {
		return errors.Wrap(err, "Chrome login failed")
	}
	defer cr.Close(ctx)

	testing.ContextLog(ctx, "Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		testing.ContextLog(ctx, "Got SessionStateChanged signal")
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get SessionStateChanged signal")
	}
	return nil
}

// readTraceMaskAndSize reads the drm_trace mask and per-cpu buffer size from sysfs.
func readTraceMaskAndSize() (mask, size int, err error) {
	mask, err = readFileToInt(traceMaskPath)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to read trace mask")
	}
	size, err = readFileToInt(bufferSizePath)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to read trace buffer size")
	}
	return mask, size, nil
}

// setTraceCategoriesAndSize sets the DRMTrace categories and mask using the DRMTraceSet* D-Bus methods on the org.chromium.debugd interface.
func setTraceCategoriesAndSize(ctx context.Context, d *debugd.Debugd, categories debugd.DRMTraceCategories, size debugd.DRMTraceSize) error {
	if err := d.DRMTraceSetCategories(ctx, categories); err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetCategories")
	}
	if err := d.DRMTraceSetSize(ctx, size); err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetSize")
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
