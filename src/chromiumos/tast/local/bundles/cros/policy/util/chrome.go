// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policy/fakedms"
)

// ResetChromeAndPolicies resets chrome and removes all set policies.
func ResetChromeAndPolicies(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome) error {
	if err := cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}

	if err := UpdatePolicyBlob(ctx, fdms, cr, fakedms.NewPolicyBlob()); err != nil {
		return errors.Wrap(err, "failed to clear policies")
	}

	return nil
}
