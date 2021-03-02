// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebUIJSErrors,
		Desc:         "Checks that Chrome's WebUI JavaScript Error Reporting works on Chrome OS",
		Contacts:     []string{"iby@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
	})
}

// checkJavaScriptError checks that a JavaScript error report was created.
// It confirms that an error report with a JavaScript stack file (.js_stack) was
// created in the crash directories crashDir. It confirms that the meta file
// contains the line "upload_var_error_message=" followed by the
// expectedErrorMessage and that the stack contains the expectedStackEntries in
// the order given.
func checkJavaScriptError(ctx, cleanupCtx context.Context, crashDir, outDir, expectedErrorMessage string,
	expectedStackEntries []string) error {
	const (
		metaRegex  = `jserror\.\d{8}\.\d{6}\.\d+\.\d+\.meta`
		stackRegex = `jserror\.\d{8}\.\d{6}\.\d+\.\d+\.js_stack`
		logRegex   = `jserror\.\d{8}\.\d{6}\.\d+\.\d+\.chrome.txt.gz`

		expectedType               = "upload_var_type=JavascriptError"
		expectedPayloadPrefix      = "payload="
		expectedLogPrefix          = "upload_file_chrome.txt="
		expectedErrorMessagePrefix = "upload_var_error_message="
	)

	crashFileMap, err := crash.WaitForCrashFiles(ctx, []string{crashDir}, []string{metaRegex, stackRegex, logRegex})
	if err != nil {
		return errors.Wrapf(err, "WaitForCrashFiles failed for directory %s", crashDir)
	}
	defer crash.RemoveAllFiles(cleanupCtx, crashFileMap)

	metas := crashFileMap[metaRegex]
	stacks := crashFileMap[stackRegex]
	logs := crashFileMap[logRegex]
	// We should have only generated one error report for each JavaScript error
	// test. We don't expect multiple error reports for a single JavaScript error.
	// This should be safe since we're only navigating to a single page so
	// there shouldn't be outside JavaScript errors. (Any errors from the login
	// page would have been removed when we did SetUpCrashTest) Only JavaScript
	// errors should produce error reports that start with jserror so we aren't
	// vulnerable to unrelated selinux violations or segfaults confusing us.
	if len(metas) != 1 || len(stacks) != 1 || len(logs) != 1 {
		crash.MoveFilesToOut(ctx, outDir, metas...)
		crash.MoveFilesToOut(ctx, outDir, stacks...)
		crash.MoveFilesToOut(ctx, outDir, logs...)
		return errors.New("multiple JS Errors found")
	}

	metaContents, err := ioutil.ReadFile(metas[0])
	if err != nil {
		return errors.Wrap(err, "couldn't read meta file")
	}

	if !strings.Contains(string(metaContents), expectedErrorMessagePrefix+expectedErrorMessage) {
		crash.MoveFilesToOut(ctx, outDir, metas[0])
		return errors.Errorf("didn't find expected meta var: %q. Leaving for debugging: %s", expectedErrorMessagePrefix+expectedErrorMessage, metas[0])
	}
	if !strings.Contains(string(metaContents), expectedType) {
		crash.MoveFilesToOut(ctx, outDir, metas[0])
		return errors.Errorf("didn't find expected meta var: %q. Leaving for debugging: %s", expectedType, metas[0])
	}
	expectedPayload := expectedPayloadPrefix + filepath.Base(stacks[0])
	if !strings.Contains(string(metaContents), expectedPayload) {
		crash.MoveFilesToOut(ctx, outDir, metas[0])
		return errors.Errorf("didn't find expected meta payload: %q. Leaving for debugging: %s", expectedPayload, metas[0])
	}
	expectedLogUpload := expectedLogPrefix + filepath.Base(logs[0])
	if !strings.Contains(string(metaContents), expectedLogUpload) {
		crash.MoveFilesToOut(ctx, outDir, metas[0])
		return errors.Errorf("didn't find expected meta log upload: %q. Leaving for debugging: %s", expectedLogUpload, metas[0])
	}

	stackContents, err := ioutil.ReadFile(stacks[0])
	if err != nil {
		return errors.Wrap(err, "couldn't read stack file")
	}
	previousLocation := -1
	for _, expectedStackEntry := range expectedStackEntries {
		location := strings.Index(string(stackContents), expectedStackEntry)
		if location == -1 {
			crash.MoveFilesToOut(ctx, outDir, stacks[0])
			return errors.Errorf("didn't find expected text in stack: %q. Leaving for debugging: %s", expectedStackEntry, stacks[0])
		}
		if location < previousLocation {
			return errors.Errorf("stack is out of order; %q too early. Leaving for debugging: %s", expectedStackEntry, stacks[0])
		}
		previousLocation = location
	}

	return nil
}

func checkPageLoadError(ctx, cleanupCtx context.Context, crashDir, outDir string) error {
	testing.ContextLog(ctx, "Checking for crash files after bad page load")

	const (
		expectedFunction1    = "logsErrorDuringPageLoadInner"
		expectedFunction2    = "logsErrorDuringPageLoadOuter"
		expectedErrorMessage = "WebUI JS Error: printing error on page load"
	)
	if err := checkJavaScriptError(ctx, cleanupCtx, crashDir, outDir, expectedErrorMessage, []string{expectedFunction1, expectedFunction2}); err != nil {
		return err
	}

	return nil
}

func checkLoggedError(ctx, cleanupCtx context.Context, crashDir, outDir string) error {
	testing.ContextLog(ctx, "Checking for crash files after JavaScripts logs an error")

	// Alt+L presses the "Log Error" button on chrome://webuijserror
	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard handle")
	}
	defer kw.Close()
	if err := kw.Accel(ctx, "Alt+l"); err != nil {
		return errors.Wrap(err, "failed to press keys")
	}

	const (
		expectedFunction1    = "logsErrorFromButtonClickInner"
		expectedFunction2    = "logsErrorFromButtonClickHandler"
		expectedErrorMessage = "WebUI JS Error: printing error on button click"
	)

	if err := checkJavaScriptError(ctx, cleanupCtx, crashDir, outDir, expectedErrorMessage, []string{expectedFunction1, expectedFunction2}); err != nil {
		return err
	}

	return nil
}

func checkUncaughtExceptionError(ctx, cleanupCtx context.Context, crashDir, outDir string) error {
	testing.ContextLog(ctx, "Checking for crash files after JavaScripts throws an unhandled exception")

	// Alt+T presses the "Throw Uncaught Error" button on chrome://webuijserror
	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard handle")
	}
	defer kw.Close()
	if err := kw.Accel(ctx, "Alt+t"); err != nil {
		return errors.Wrap(err, "failed to press keys")
	}

	const (
		expectedFunction1    = "throwExceptionInner"
		expectedFunction2    = "throwExceptionHandler"
		expectedErrorMessage = "Uncaught Error: WebUI JS Error: exception button clicked"
	)

	if err := checkJavaScriptError(ctx, cleanupCtx, crashDir, outDir, expectedErrorMessage, []string{expectedFunction1, expectedFunction2}); err != nil {
		return err
	}

	return nil
}

func checkUnhandledPromiseRejectionError(ctx, cleanupCtx context.Context, crashDir, outDir string) error {
	testing.ContextLog(ctx, "Checking for crash files after JavaScripts doesn't handle a promise rejection")

	// Alt+P presses the "Unhandled Promise Rejection" button on chrome://webuijserror
	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard handle")
	}
	defer kw.Close()
	if err := kw.Accel(ctx, "Alt+p"); err != nil {
		return errors.Wrap(err, "failed to press keys")
	}

	const (
		expectedStack        = "No Stack"
		expectedErrorMessage = "Uncaught (in promise) WebUI JS Error: The rejector always rejects!"
	)

	if err := checkJavaScriptError(ctx, cleanupCtx, crashDir, outDir, expectedErrorMessage, []string{expectedStack}); err != nil {
		return err
	}

	return nil
}

func WebUIJSErrors(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	const vModuleFlags = "--vmodule=chrome_js_error_report_processor=3,web_ui_impl=3,web_ui_main_frame_observer=3,webui_js_error_ui=3"
	cr, err := chrome.New(ctx, chrome.EnableFeatures("SendWebUIJavaScriptErrorReports"), chrome.ExtraArgs(vModuleFlags))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(cleanupCtx)

	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(cleanupCtx)

	conn, err := cr.NewConn(ctx, "chrome://webuijserror")
	if err != nil {
		s.Fatal("Chrome navigation failed: ", err)
	}
	defer conn.Close()

	user := cr.NormalizedUser()
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		s.Fatal("Couldn't get user path: ", err)
	}
	crashDir := filepath.Join(path, "crash")

	if err := checkPageLoadError(ctx, cleanupCtx, crashDir, s.OutDir()); err != nil {
		s.Error("checkPageLoadError failed: ", err)
	}
	if err := checkLoggedError(ctx, cleanupCtx, crashDir, s.OutDir()); err != nil {
		s.Error("checkLoggedError failed: ", err)
	}
	if err := checkUncaughtExceptionError(ctx, cleanupCtx, crashDir, s.OutDir()); err != nil {
		s.Error("checkUncaughtExceptionError failed: ", err)
	}
	if err := checkUnhandledPromiseRejectionError(ctx, cleanupCtx, crashDir, s.OutDir()); err != nil {
		s.Error("checkUnhandledPromiseRejectionError failed: ", err)
	}

}
