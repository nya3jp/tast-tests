// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// VerifyNoLaucherApp verifies that an app does not have an icon present so that
// it cannot be launched.
func VerifyNoLaucherApp(ctx context.Context, s *testing.State, ownerID, appName, appID string) {
	s.Log("Checking that app icons do not exist for ", appName)
	checkIconNonExistence(ctx, s, ownerID, appName, appID)
}

// checkIconNonExistence determines if the Crostini icon folder for the
// specified application exists in the filesystem and contains at least one
// file. It produces an error if so.
func checkIconNonExistence(ctx context.Context, s *testing.State, ownerID, appName, appID string) {
	iconDir := filepath.Join("/home/user", ownerID, "crostini.icons", appID)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		fileInfo, err := os.Stat(iconDir)
		if err != nil {
			return nil // Directory doesn't exist; success
		}
		if !fileInfo.IsDir() {
			// Should either not exist or be a directory.
			return errors.Errorf("icon path %v is not a directory", iconDir)
		}
		entries, err := ioutil.ReadDir(iconDir)
		if err != nil {
			return errors.Wrapf(err, "failed reading dir %v", iconDir)
		}
		if len(entries) > 0 {
			return errors.Errorf("icons exist in %v", iconDir)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	if err != nil {
		s.Errorf("Still found %v icons in %v", appName, iconDir)
	}
}
