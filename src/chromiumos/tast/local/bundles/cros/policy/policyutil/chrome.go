// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// ResetChrome resets chrome and removes all policies previously served by the FakeDMS.
func ResetChrome(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome) error {
	if err := cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome")
	}

	if err := ServeBlobAndRefresh(ctx, fdms, cr, fakedms.NewPolicyBlob()); err != nil {
		return errors.Wrap(err, "failed to clear policies")
	}

	return nil
}
