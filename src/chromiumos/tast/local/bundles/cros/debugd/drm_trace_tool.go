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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
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
		SoftwareDeps: []string{"chrome", "drm_trace"},
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
	log := fmt.Sprintf("annotate-%d", time.Now().Unix())
	err = testAnnotate(ctx, obj, log)
	if err != nil {
		s.Error("Failed to verify DRMTraceAnnotateLog: ", err)
	}

	s.Log("Verify DRMTraceSetCategories method")
	const testCategories = drmTraceCategoryCore | drmTraceCategoryKMS
	err = testSetCategories(ctx, obj, testCategories, testCategories)
	if err != nil {
		s.Error("Failed to verify DRMTraceSetCategories: ", err)
	}

	const (
		defaultCategories         = 0
		expectedDefaultCategories = drmTraceCategoryDriver | drmTraceCategoryKMS | drmTraceCategoryDP
	)
	err = testSetCategories(ctx, obj, defaultCategories, expectedDefaultCategories)
	if err != nil {
		s.Error("Failed to verify DRMTraceSetCategories: ", err)
	}

	s.Log("Verify DRMTraceSetSize method")
	err = testSetSize(ctx, obj)
	if err != nil {
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
	err = testLogin(ctx, obj, s, sm)
	if err != nil {
		s.Error("Failed to verify DRMTraceTool parameter reset on login: ", err)
	}
	err = testLogout(ctx, obj, s, sm)
	if err != nil {
		s.Error("Failed to verify DRMTraceTool parameter reset on logout: ", err)
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

// testLogin verifies that drm_trace related configuration is reset correctly on login.
func testLogin(ctx context.Context, obj dbus.BusObject, s *testing.State, sm *session.SessionManager) error {
	// Record the current (default) values for the mask and size.
	defaultMask, defaultSize, err := getTraceMaskAndSize()
	if err != nil {
		return err
	}

	// Before login, set the mask and buffer size to some non-default values.
	const testCategories uint32 = drmTraceCategoryCore | drmTraceCategoryKMS
	setTraceMaskAndSize(ctx, obj, testCategories, drmTraceSizeDebug)
	if err != nil {
		return err
	}

	// Log in to Chrome, and wait for login to be complete.
	err = chromeLogin(ctx, s, sm)
	if err != nil {
		return err
	}

	// Get the mask and size after login is complete.
	afterLoginMask, afterLoginSize, err := getTraceMaskAndSize()
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
func testLogout(ctx context.Context, obj dbus.BusObject, s *testing.State, sm *session.SessionManager) error {
	// Log in to Chrome, and wait for login to be complete.
	err := chromeLogin(ctx, s, sm)
	if err != nil {
		return err
	}

	// Record the current (default) values for the mask and size.
	defaultMask, defaultSize, err := getTraceMaskAndSize()
	if err != nil {
		return err
	}

	// Before logout, set the mask and buffer size to some non-default values.
	const testCategories uint32 = drmTraceCategoryCore | drmTraceCategoryKMS
	setTraceMaskAndSize(ctx, obj, testCategories, drmTraceSizeDebug)
	if err != nil {
		return err
	}

	// Emulate a logout by restarting "ui".
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// Get the mask and size after login is complete.
	afterLogoutMask, afterLogoutSize, err := getTraceMaskAndSize()
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
func chromeLogin(ctx context.Context, s *testing.State, sm *session.SessionManager) error {
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

	s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get SessionStateChanged signal")
	}
	return nil
}

func getTraceMaskAndSize() (int, int, error) {
	size, err := readFileToInt(bufferSizePath)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to read trace buffer size")
	}
	mask, err := readFileToInt(traceMaskPath)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to read trace mask")
	}
	return size, mask, nil
}

func setTraceMaskAndSize(ctx context.Context, obj dbus.BusObject, categoriesMask, sizeEnum uint32) error {
	if err := obj.CallWithContext(ctx, dbusInterface+".DRMTraceSetSize", 0, sizeEnum).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetSize")
	}
	if err := obj.CallWithContext(ctx, dbusInterface+".DRMTraceSetCategories", 0, categoriesMask).Err; err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSetCategories")
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
