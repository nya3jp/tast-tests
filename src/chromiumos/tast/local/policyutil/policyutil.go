// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/timing"
)

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
