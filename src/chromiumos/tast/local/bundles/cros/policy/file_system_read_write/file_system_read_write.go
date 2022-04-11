// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesystemreadwrite

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	// SaveFilePicker is a link's name which can save file.
	SaveFilePicker = "showSaveFilePicker"
	// SaveFileDialog is save file dialog's name.
	SaveFileDialog = "Save file as"

	shortTimeout = 5 * time.Second
	longTimeout  = 20 * time.Second
)

// Values for the DefaultFileSystemGuardSetting policy.
const (
	DefaultGuardSettingBlock = 2
	DefaultGuardSettingAsk   = 3
)

// AccessMethod represents the access method to file system.
type AccessMethod int

const (
	// Write is the write access to the file system.
	Write AccessMethod = iota
)

// TestCase represents the name of test, the policy to use.
type TestCase struct {
	// Name is the test case's name.
	Name string
	// URL is the website's URL the test case will connect to.
	URL string
	// WantFileSystemWrite is whether the test case have write access.
	WantFileSystemWrite bool
	// Method defines the access method to file system.
	Method AccessMethod
	// Policies is the policies the test case sets.
	Policies []policy.Policy
}

// triggerFilePicker clicks file picker.
func triggerFilePicker(ctx context.Context, conn *chrome.Conn, ui *uiauto.Context, dialogName, filePicker string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		// Attempt to open the file picker by clicking the HTML link that triggers
		// window.showSaveFilePicker(). We cannot use conn.Eval() for this,
		// because opening the file picker must be triggered by a user gesture for
		// security reasons.
		if err := ui.LeftClick(nodewith.Role(role.Link).Name(filePicker))(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to click link to open the save file picker"))
		}

		// Wait until opening the file picker has either succeeded or failed.
		var errorMessage string
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := conn.Eval(ctx, "window.errorMessage", &errorMessage); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to evaluate window.errorMessage"))
			}

			// If the error message is empty, this could mean one of two things:
			// 1. The asynchronous opening of the file picker has neither succeeded nor failed yet.
			// 2. The file picker has successfully opened, and the `await window.showSaveFilePicker` call is waiting for the user to select a file
			if errorMessage == "" {
				// Continue polling if the dialog hasn't yet opened.
				return ui.Exists(nodewith.Role(role.Dialog).Name(dialogName).ClassName("RootView"))(ctx)
			}

			// The error message is non-empty, thus opening the file picker has failed; stop polling.
			return nil
		}, &testing.PollOptions{Timeout: shortTimeout}); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to wait for the file picker to either open or fail to open"))
		}

		if errorMessage == "User activation is required to show a file picker." {
			// Sometimes Chrome will not register the click as a user gesture, and thus not open the file picker.
			// Return an error here so that we retry the click to open the file picker.
			return errors.New("failed to open the file picker: The click action was not recognized as a user gesture")
		} else if errorMessage != "" {
			testing.ContextLogf(ctx, "Opening the file picker failed with the following error: %s", errorMessage)
		}

		return nil
	}, &testing.PollOptions{Timeout: longTimeout})
}

// checkFileContent waits for the file to be written to disk and checks its contents.
func checkFileContent(ctx context.Context, filePath string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		fileContent, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}
		if string(fileContent) != "test" {
			return errors.Errorf("File contains invalid content, expected 'test', got: %s", string(fileContent))
		}
		return nil
	}, &testing.PollOptions{Timeout: longTimeout})
}

// RunTestCases setups the device and runs compat test cases.
func RunTestCases(ctx context.Context, s *testing.State, param TestCase) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Update policies.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.Policies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users MyFiles path: ", err)
	}

	// Cleanup the file _after_ the browser has been closed, to be sure that
	// the browser is not still in the process of writing the file.
	filePath := path.Join(myFilesPath, "test-file")
	if param.WantFileSystemWrite {
		defer os.Remove(filePath)
	}

	// Setup browser based on the browser type.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	conn, err := br.NewConn(ctx, param.URL)
	if err != nil {
		s.Fatal("Failed to open website: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, param.Name)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	switch param.Method {
	case Write:
		if err := triggerFilePicker(ctx, conn, ui, SaveFileDialog, SaveFilePicker); err != nil {
			s.Fatal("Failed to trigger file picker: ", err)
		}
		// At this point, the file picker has either opened, or failed to open with an error.

		if param.WantFileSystemWrite {
			if err := ui.LeftClick(nodewith.Role(role.Button).Name("Save"))(ctx); err != nil {
				s.Fatal("Failed to save file using save file picker: ", err)
			}

			if err := checkFileContent(ctx, filePath); err != nil {
				s.Fatal("Failed to check file content: ", err)
			}
		} else {
			if err := ui.EnsureGoneFor(nodewith.Role(role.Dialog).Name(SaveFileDialog).HasClass("RootView"), shortTimeout)(ctx); err != nil {
				s.Error("Save file picker opened unexpectedly")
			}
		}
	default:
		s.Error("Unexpected method: ", param.Method)
	}

}
