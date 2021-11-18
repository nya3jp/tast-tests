// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// dumpPolicies saves the policies that are availabled on DUT in json format.
// fileLocation is expected to be a canonical, absolute path to a file to
// store policies.
func dumpPolicies(ctx context.Context, tconn *chrome.TestConn, fileLocation string) error {
	policies, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain policies from Chrome")
	}

	b, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal policies")
	}

	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		return errors.New("output directory unavailable")
	}

	// Dump all policies as seen by Chrome to the tests OutDir.
	if err := ioutil.WriteFile(filepath.Join(dir, fileLocation), b, 0644); err != nil {
		return errors.Wrap(err, "failed to dump policies to file")
	}
	return nil
}
