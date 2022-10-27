// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
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
	bufferSizePath    = "/sys/kernel/tracing/instances/drm/buffer_size_kb"
	traceContentsPath = "/sys/kernel/tracing/instances/drm/trace"
	traceMaskPath     = "/sys/module/drm/parameters/trace"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DRMTraceTool,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests D-Bus methods related to DRMTraceTool",
		Contacts: []string{
			"ddavenport@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "drm_trace"},
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraSoftwareDeps: []string{"no_kernel_upstream"},
			},
			{
				Name:      "unstable",
				ExtraAttr: []string{"informational"},
			},
		},
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

	s.Log("Verify DRMTraceSnapshot method")
	if err := testTraceSnapshot(ctx, dbgd); err != nil {
		s.Error("Failed to verify DRMTraceSnapshot for drm_trace: ", err)
	}
	if err := testModetestSnapshot(ctx, dbgd); err != nil {
		s.Error("Failed to verify DRMTraceSnapshot for modetest: ", err)
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
		return errors.Errorf("unexpected value for trace mask: got %d, want %d", mask, expected)
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

// testTraceSnapshot verifies the DRMTraceSnapshot D-Bus method for debugd.DRMTraceSnapshotTypeTrace
func testTraceSnapshot(ctx context.Context, d *debugd.Debugd) error {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("couldn't get output dir")
	}

	// Clear any snapshots from previous runs.
	if err := removeDirContents("/var/log/display_debug"); err != nil {
		return errors.Wrap(err, "failed to clear /var/log/display_debug directory")
	}

	// Turn off trace categories to control the log contents during the test case.
	if err := writeStringToFile(traceMaskPath, "0"); err != nil {
		return errors.Wrap(err, "failed to disable trace mask")
	}

	// Do the snapshot.
	if err := d.DRMTraceSnapshot(ctx, debugd.DRMTraceSnapshotTypeTrace); err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSnapshot")
	}

	// Read the trace log.
	trace, err := readFileToString(traceContentsPath)
	if err != nil {
		return errors.Wrap(err, "failed to read trace contents")
	}

	// Read the snapshot.
	matches, err := filepath.Glob("/var/log/display_debug/drm_trace_verbose.*")
	if err != nil {
		return errors.Wrapf(err, "failed to glob directory: %s", "/var/log/display_debug/drm_trace_verbose.*")
	}
	if len(matches) != 1 {
		return errors.Errorf("unexpected number of glob matches: got %d want 1", len(matches))
	}

	snapshot, err := readFileToString(matches[0])
	if err != nil {
		return errors.Wrap(err, "failed to read snapshot contents")
	}

	if !strings.HasSuffix(trace, snapshot) {
		if err := ioutil.WriteFile(filepath.Join(dir, "snapshot"), []byte(snapshot), 0644); err != nil {
			testing.ContextLog(ctx, "Failed to write snapshot file to output dir")
		}
		if err := ioutil.WriteFile(filepath.Join(dir, "trace"), []byte(trace), 0644); err != nil {
			testing.ContextLog(ctx, "Failed to write snapshot file to output dir")
		}
		return errors.New("trace and snapshot contents do not match")
	}

	return nil
}

// testModetestSnapshot verifies the DRMTraceSnapshot D-Bus method for debugd.DRMTraceSnapshotTypeModetest
func testModetestSnapshot(ctx context.Context, d *debugd.Debugd) error {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("couldn't get output dir")
	}

	// Clear any snapshots from previous runs.
	if err := removeDirContents("/var/log/display_debug"); err != nil {
		return errors.Wrap(err, "failed to clear /var/log/display_debug directory")
	}

	// Do the snapshot.
	if err := d.DRMTraceSnapshot(ctx, debugd.DRMTraceSnapshotTypeModetest); err != nil {
		return errors.Wrap(err, "failed to call DRMTraceSnapshot")
	}

	// Read the snapshot.
	matches, err := filepath.Glob("/var/log/display_debug/modetest.*")
	if err != nil {
		return errors.Wrapf(err, "failed to glob directory: %s", "/var/log/display_debug/modetest.*")
	}
	if len(matches) != 1 {
		return errors.Errorf("unexpected number of glob matches: got %d want 1", len(matches))
	}

	snapshot, err := readFileToString(matches[0])
	if err != nil {
		return errors.Wrap(err, "failed to read snapshot contents")
	}

	// Check if it looks like reasonable modetest output.
	var reason string
	if len(snapshot) == 0 {
		reason = "snapshot is empty"
	} else if match, _ := regexp.Match("(?m)^Connectors:", []byte(snapshot)); !match {
		reason = "did not find Connectors section"
	} else if match, _ := regexp.Match("(?m)^CRTCs:", []byte(snapshot)); !match {
		reason = "did not find CRTCs section"
	} else if match, _ := regexp.Match("(?m)^Planes:", []byte(snapshot)); !match {
		reason = "did not find Planes section"
	} else {
		// modetest output looks reasonable, return with no error.
		return nil
	}

	// Fall through to error handling.
	if err := ioutil.WriteFile(filepath.Join(dir, "modetest_snapshot"), []byte(snapshot), 0644); err != nil {
		testing.ContextLog(ctx, "Failed to write snapshot file to output dir")
	}
	return errors.Errorf("snapshot contents does not look like modetest output: %s", reason)
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
		return errors.Errorf("unexpected buffer size found: got (%d), want (%d)", afterLoginSize, defaultSize)
	}

	if afterLoginMask != defaultMask {
		return errors.Errorf("unexpected trace mask found: get (%d), want (%d)", afterLoginMask, defaultSize)
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
		return errors.Errorf("unexpected buffer size found: got (%d), want (%d)", afterLogoutSize, defaultSize)
	}

	if afterLogoutMask != defaultMask {
		return errors.Errorf("unexpected trace mask found: got (%d), want (%d)", afterLogoutMask, defaultSize)
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
	const replacementCharacter = '\uFFFD'
	return strings.TrimSpace(strings.ToValidUTF8(string(bytes), string(replacementCharacter))), nil
}

// readFileToInt opens the file at |path|, reads its contents, and returns the contents as an int.
func readFileToInt(path string) (int, error) {
	sizeStr, err := readFileToString(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(sizeStr)
}

// writeStringToFile will write |contents| to the file at |path|.
// If |path| does not exist or can't be opened for writing, this will return error.
func writeStringToFile(path, contents string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(contents)
	return err
}

// removeDirContents will remove all files and directories in the directory at |path|.
// The directory at |path| itself will not be removed. If the directory doesn't exist,
// this function succeeds trivially.
func removeDirContents(path string) error {
	dir, err := ioutil.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, d := range dir {
		if err := os.RemoveAll(filepath.Join(path, d.Name())); err != nil {
			return err
		}
	}
	return nil
}
