// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// InstallPwaAppByPolicy installs a pre-defined Progressive Web App (PWA). Returns the app's id,
// the name, the callback for cleanup and the error message if any. This method guarantees to
// do cleanup if the method execution generates an error. Callers have the responsibility to run
// the cleanup closure if the method is executed without any error.
// NOTE executing this method requires the following files:
// 1. web_app_install_force_list_icon-192x192.png
// 2. web_app_install_force_list_icon-512x512.png
// 3. web_app_install_force_list_index.html
// 4. web_app_install_force_list_manifest.json
// 5. web_app_install_force_list_service-worker.js
// These files are kept under the subdirectory data/ under the policyutil package. Callers have the duty to
// symlink these files into their test packages' data subdirectory and register them.
func InstallPwaAppByPolicy(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, fdms *fakedms.FakeDMS, root http.FileSystem) (string, string, func(ctx context.Context) error, error) {
	server := httptest.NewServer(http.FileServer(root))

	policies := []policy.Policy{
		&policy.WebAppInstallForceList{
			Val: []*policy.WebAppInstallForceListValue{
				{
					Url:                    server.URL + "/web_app_install_force_list_index.html",
					DefaultLaunchContainer: "window",
					CreateDesktopShortcut:  false,
					CustomName:             "",
					FallbackAppName:        "",
					CustomIcon: &policy.WebAppInstallForceListValueCustomIcon{
						Hash: "",
						Url:  "",
					},
				},
			},
		},
	}

	cleanUp := func(ctx context.Context) error {
		server.Close()
		if err := ResetChrome(ctx, fdms, cr); err != nil {
			return errors.Wrap(err, "failed to reset policies")
		}
		return nil
	}

	const name = "Test PWA"
	var id string
	var err error
	// TODO(b/254067263): Ensure the policy is refreshed for app installation
	// particularly on Lacros when ServeAndVerify returns.
	// Remove this polling if it makes sense once the flake is addressed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Update policies.
		if err := ServeAndVerify(ctx, fdms, cr, policies); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to update policies"))
		}
		testing.ContextLog(ctx, "Waiting for the PWA to be installed")
		id, err = ash.WaitForChromeAppByNameInstalled(ctx, tconn, name, 30*time.Second)
		if err != nil {
			testing.ContextLog(ctx, "Failed to wait until the PWA is installed, try again")
			return errors.Wrap(err, "failed to wait until the PWA is installed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 1 * time.Minute}); err != nil {
		cleanUp(ctx)
		return "", "", nil, errors.Wrap(err, "failed to update policies")
	}

	return id, name, cleanUp, nil
}

// serveAndVerify is a helper function. Similar to ServeAndVerify(OnLoginScreen) but also accepts the test connection.
func serveAndVerify(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, tconn *chrome.TestConn, ps []policy.Policy) error {
	if err := serveAndRefresh(ctx, fdms, cr, tconn, ps); err != nil {
		return errors.Wrap(err, "failed to serve policies")
	}

	return Verify(ctx, tconn, ps)
}

// ServeAndVerify serves the policies using ServeAndRefresh and verifies that they are set in Chrome.
func ServeAndVerify(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	return serveAndVerify(ctx, fdms, cr, tconn, ps)
}

// ServeAndVerifyOnLoginScreen same as ServeAndVerify but in the login context. It uses the Signin Profile Test API connection.
func ServeAndVerifyOnLoginScreen(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy) error {
	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Signin Profile Test API connection")
	}
	return serveAndVerify(ctx, fdms, cr, tconn, ps)
}

// ServeAndRefresh updates the policies served by FakeDMS and refreshes them in Chrome.
// Not all polcies can be set in this way and may require restarting Chrome or a reboot.
func ServeAndRefresh(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	return serveAndRefresh(ctx, fdms, cr, tconn, ps)
}

// serveAndRefresh is a helper function. Similar to ServeAndRefresh but also accepts the test connection.
func serveAndRefresh(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, tconn *chrome.TestConn, ps []policy.Policy) error {
	pb := policy.NewBlob()
	pb.AddPolicies(ps)
	return serveBlobAndRefresh(ctx, fdms, cr, tconn, pb)
}

// ServeBlobAndRefresh updates the policy blob of FakeDMS and refreshes the policies in Chrome.
func ServeBlobAndRefresh(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, pb *policy.Blob) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}
	return serveBlobAndRefresh(ctx, fdms, cr, tconn, pb)
}

// serveBlobAndRefresh is a helper function. Similar to ServeBlobAndRefresh but also accepts the test connection.
func serveBlobAndRefresh(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, tconn *chrome.TestConn, pb *policy.Blob) error {
	// Make sure FakeDMS is still running.
	if err := fdms.Ping(ctx); err != nil {
		return errors.Wrap(err, "failed to ping FakeDMS")
	}

	if err := fdms.WritePolicyBlob(pb); err != nil {
		return errors.Wrap(err, "failed to write policies to FakeDMS")
	}

	if err := Refresh(ctx, tconn); err != nil {
		return err
	}

	return nil
}

// RefreshChromePolicies forces an immediate refresh of policies in Chrome.
func RefreshChromePolicies(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	return Refresh(ctx, tconn)
}

// ResetChrome resets chrome and removes all policies previously served by the FakeDMS.
func ResetChrome(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome) error {
	return ResetChromeWithBlob(ctx, fdms, cr, policy.NewBlob())
}

// ResetChromeWithBlob resets chrome and replaces all policies previously served by the FakeDMS with PolicyBlob.
func ResetChromeWithBlob(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, pb *policy.Blob) error {
	ctx, cancel := context.WithTimeout(ctx, chrome.ResetTimeout)
	defer cancel()

	if err := cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "failed to communicate with Chrome")
	}

	if err := cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset Chrome")
	}

	if err := ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		return errors.Wrap(err, "failed to clear policies")
	}

	return nil
}

// Refresh takes a running Chrome API connection and refreshes policies.
// If the policices served have changed between now and the last time policies
// were fetched, this function will ensure that Chrome uses the changes.
// Note that this will not work for policies which require a reboot before a
// change is applied.
func Refresh(ctx context.Context, tconn *chrome.TestConn) error {
	ctx, st := timing.Start(ctx, "policy_refresh")
	defer st.End()

	return tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)()`, nil)
}

// SetUpFakePolicyServer creates a FakeDMS that enforces the provided policies.
func SetUpFakePolicyServer(ctx context.Context, outdir, policyUser string, policies []policy.Policy) (fdms *fakedms.FakeDMS, retErr error) {
	fdms, err := fakedms.New(ctx, outdir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create fakedms")
	}
	defer func() {
		if retErr != nil {
			fdms.Stop(ctx)
		}
	}()

	// Add the new policies to fmds.
	blob := policy.NewBlob()
	blob.PolicyUser = policyUser
	if err := blob.AddPolicies(policies); err != nil {
		return nil, errors.Wrap(err, "failed to add policy to policy blob")
	}
	if err := fdms.WritePolicyBlob(blob); err != nil {
		return nil, errors.Wrap(err, "failed to write policy blob to fdms")
	}
	return fdms, nil
}
