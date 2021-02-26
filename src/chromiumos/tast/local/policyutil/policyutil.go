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
	"chromiumos/tast/testing"
)

// ServeAndVerify serves the policies using ServeAndRefresh and verifies that they are set in Chrome.
func ServeAndVerify(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy) error {
	if err := ServeAndRefresh(ctx, fdms, cr, ps); err != nil {
		return errors.Wrap(err, "failed to serve policies")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	return Verify(ctx, tconn, ps)
}

// ServeAndRefresh updates the policies served by FakeDMS and refreshes them in Chrome.
// Not all polcies can be set in this way and may require restarting Chrome or a reboot.
func ServeAndRefresh(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy) error {
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(ps)
	return ServeBlobAndRefresh(ctx, fdms, cr, pb)
}

// ServeBlobAndRefresh updates the policy blob of FakeDMS and refreshes the policies in Chrome.
func ServeBlobAndRefresh(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, pb *fakedms.PolicyBlob) error {
	// Make sure FakeDMS is still running.
	if err := fdms.Ping(ctx); err != nil {
		return errors.Wrap(err, "failed to ping FakeDMS")
	}

	if err := fdms.WritePolicyBlob(pb); err != nil {
		return errors.Wrap(err, "failed to write policies to FakeDMS")
	}

	if err := RefreshChromePolicies(ctx, cr); err != nil {
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

	// Refresh policies and make sure Chrome is still valid.
	if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)()`, nil); err != nil {
		return errors.Wrap(err, "failed to refresh policies")
	}

	return nil
}

// ResetChrome resets chrome and removes all policies previously served by the FakeDMS.
func ResetChrome(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome) error {
	ctx, cancel := context.WithTimeout(ctx, chrome.ResetTimeout)
	defer cancel()

	if err := cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "failed to communicate with Chrome")
	}

	const MaxAttempts = 3
	attempt := 1
	var rsErr error
	// Sometimes resetting Chrome state fails. This retries it up to MaxAttempts.
	for attempt <= MaxAttempts {
		rsErr = cr.ResetState(ctx)
		if rsErr != nil {
			testing.ContextLogf(ctx, "WARNING - %d. Chrome reset state attempt failed due to %v Retrying", attempt, rsErr)
		} else {
			break
		}
	}

	if rsErr != nil {
		return errors.Wrapf(rsErr, "failed to reset Chrome in %d attempts", MaxAttempts)
	}

	if err := ServeBlobAndRefresh(ctx, fdms, cr, fakedms.NewPolicyBlob()); err != nil {
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
	return tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)()`, nil)
}
