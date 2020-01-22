// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policy"
	"chromiumos/tast/local/policy/fakedms"
)

// UpdatePolicies updates the polies of FakeDMS and refreshes them in Chrome.
func UpdatePolicies(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy) error {
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(ps)
	return UpdateBlob(ctx, fdms, cr, pb)
}

// UpdateBlob updates the policy blob of FakeDMS and refreshes them in Chrome.
func UpdateBlob(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, pb *fakedms.PolicyBlob) error {
	// Make sure FakeDMS is still running.
	if err := fdms.Ping(ctx); err != nil {
		return errors.Wrap(err, "failed to ping FakeDMS")
	}

	if err := fdms.WritePolicyBlob(pb); err != nil {
		return errors.Wrap(err, "failed to write policies to FakeDMS")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// Refresh policies and make sure Chrome is still sane.
	result := false
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)().then(() => true);`, &result); err != nil {
		return errors.Wrap(err, "failed to refresh policies")
	}

	if !result {
		return errors.New("eval 'true' returned false")
	}

	return nil
}
