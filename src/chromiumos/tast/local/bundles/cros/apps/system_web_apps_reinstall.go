// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"bytes"
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemWebAppsReinstall,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that system web apps can be reinstalled",
		Contacts: []string{
			"qjw@chromium.org", // Test author
			"chrome-apps-platform-rationalization@google.com", // Backup mailing list
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

// SystemWebAppsReinstall tests that system web apps can be reinstalled (i.e. don't crash Chrome).
func SystemWebAppsReinstall(ctx context.Context, s *testing.State) {
	testFileContent := make([]byte, 8)
	rand.Read(testFileContent)

	// First session: fresh install.
	err := func() error {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		cr, tconn, err := createChrome(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create Chrome instance")

		}

		var didSignOut bool
		defer func() {
			cleanup(cleanupCtx, didSignOut, tconn)
		}()

		// Create a test file for confidence check during second chrome session.
		testFilePath, err := testFilePath(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to get test file path")
		}
		if err := ioutil.WriteFile(testFilePath, testFileContent, 0644); err != nil {
			return errors.Wrap(err, "failed to create test file for confidence check")
		}

		if err := checkAppsInstalled(ctx, tconn); err != nil {
			return errors.Wrap(err, "apps aren't installed")
		}

		if err := quicksettings.SignOut(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to sign out")
		}

		didSignOut = true
		return nil
	}()

	if err != nil {
		s.Fatal("Ran into a failure during first Chrome session (system web apps fresh install): ", err)
	}

	// Second session: keep state and trigger reinstall.
	err = func() error {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		cr, tconn, err := createChrome(ctx, chrome.KeepState(), chrome.EnableFeatures("AlwaysReinstallSystemWebApps"))
		if err != nil {
			return errors.Wrap(err, "failed to create Chrome instance")
		}

		var didSignOut bool
		defer func() {
			cleanup(cleanupCtx, didSignOut, tconn)
		}()

		// Confidence check: confirm the state is persisted.
		testFilePath, err := testFilePath(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to get test file path")
		}

		defer os.Remove(testFilePath)

		b, err := ioutil.ReadFile(testFilePath)
		if err != nil {
			return errors.Wrap(err, "failed to pass confidence check")
		}

		if !bytes.Equal(testFileContent, b) {
			return errors.Errorf("failed to pass confidence check, content differs, want %v, got: %v", testFileContent, b)
		}

		if err := checkAppsInstalled(ctx, tconn); err != nil {
			return errors.Wrap(err, "apps aren't installed")
		}

		if err := quicksettings.SignOut(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to sign out")
		}

		didSignOut = true
		return nil
	}()

	if err != nil {
		s.Fatal("Ran into a failure during second Chrome session (system web apps reinstall): ", err)
	}
}

// createChrome creates a new Chrome instance with `chromeOpts` and returns Chrome and its Test API connection.
func createChrome(ctx context.Context, chromeOpts ...chrome.Option) (*chrome.Chrome, *chrome.TestConn, error) {
	// Create a fresh login.
	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create Chrome")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to Test API")
	}

	return cr, tconn, nil
}

// checkAppsInstalled performs assertions and verifies a set of system web apps that covers different
// install code paths are installed by querying AppService.
func checkAppsInstalled(ctx context.Context, tconn *chrome.TestConn) error {
	registeredSystemWebApps, err := apps.ListRegisteredSystemWebApps(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get registered system web apps")
	}

	// A set of system web apps that trigger different install code paths.
	testAppInternalNames := map[string]bool{
		"OSSettings": true,
		"Media":      true,
		"Help":       true,
	}

	for _, swa := range registeredSystemWebApps {
		if testAppInternalNames[swa.InternalName] {
			app, err := apps.FindSystemWebAppByOrigin(ctx, tconn, swa.StartURL)
			if err != nil {
				return errors.Wrapf(err, "failed to match origin, app: %s, origin: %s", swa.InternalName, swa.StartURL)
			}
			if app == nil {
				return errors.Errorf("failed to find system web app that should have been installed: %s", swa.InternalName)
			}
		}
	}

	return nil
}

// testFilePath return a path in user's Downloads folder for confidence check.
func testFilePath(ctx context.Context, cr *chrome.Chrome) (string, error) {
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve user's Downloads path")
	}

	return filepath.Join(downloadsPath, "system-web-apps-reinstall-confidence-check"), nil
}

// cleanup signs out if the test hasn't already done so.
// Note, this function shouldn't call s.Fatal() or s.FailNow() because doing so will "hide" the real error occurred during the test.
func cleanup(ctx context.Context, didSignOut bool, tconn *chrome.TestConn) {
	if didSignOut {
		return
	}

	if err := quicksettings.SignOut(ctx, tconn); err != nil {
		testing.ContextLog(ctx, "Failed to sign out during cleanup: ", err)
		testing.ContextLog(ctx, "The above error is likely caused by an error occurred in test body")
	}
}
