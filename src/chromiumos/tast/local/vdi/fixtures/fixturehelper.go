// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
)

func storePolicies(ctx context.Context, tconn *chrome.TestConn, fileLocation string) error {
	policies, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain policies from Chrome")
	}

	b, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal policies")
	}

	// Dump all policies as seen by Chrome to the tests OutDir.
	if err := ioutil.WriteFile(fileLocation, b, 0644); err != nil {
		return errors.Wrap(err, "failed to dump policies to file")
	}
	return nil
}
