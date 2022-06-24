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

		// Schedule a sign out, prepare for the next login.
		defer func(ctx context.Context, tconn *chrome.TestConn) {
			if err := quicksettings.SignOut(ctx, tconn); err != nil {
				s.Fatal("Failed to sign-out: ", err)
			}
		}(cleanupCtx, tconn)

		// Create a test file for confidence check during second chrome session.
		testFilePath, err := testFilePath(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to get test file path")
		}
		if err = ioutil.WriteFile(testFilePath, testFileContent, 0644); err != nil {
			return errors.Wrap(err, "failed to create test file for confidence check")
		}

		return checkAppsInstalled(ctx, tconn)
	}()

	if err != nil {
		s.Fatal("Failed to run first Chrome session: ", err)
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

		// Sign out to restore state. This test don't use fixture, and need to cleanup after itself.
		defer func(ctx context.Context, tconn *chrome.TestConn, s *testing.State) {
			if err := quicksettings.SignOut(ctx, tconn); err != nil {
				s.Fatal("Failed to sign-out: ", err)
			}
		}(cleanupCtx, tconn, s)

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

		return checkAppsInstalled(ctx, tconn)
	}()

	if err != nil {
		s.Fatal("Failed to run second Chrome session: ", err)
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
