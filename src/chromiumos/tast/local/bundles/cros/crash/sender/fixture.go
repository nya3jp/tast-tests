// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sender

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
)

// SetUp sets up environment suitable for crash_sender testing.
// cr is a logged-in chrome session. TearDown must be called later to clean up.
func SetUp(ctx context.Context, cr *chrome.Chrome) (retErr error) {
	defer func() {
		if retErr != nil {
			TearDown()
		}
	}()

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		return err
	}

	if err := EnableMock(true); err != nil {
		return err
	}

	if err := ResetSendRecords(); err != nil {
		return err
	}

	return nil
}

// TearDown cleans up environment set up by SetUp.
func TearDown() error {
	var firstErr error
	if err := DisableMock(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := crash.TearDownCrashTest(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
