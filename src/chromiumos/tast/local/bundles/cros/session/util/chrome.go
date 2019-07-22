// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"

	"chromiumos/tast/local/session"
)

// PrepareChromeForTesting prepares Chrome for common tests.
// This prevents a crash on startup due to synchronous profile creation and not
// knowing whether to expect policy, see https://crbug.com/950812.
func PrepareChromeForTesting(ctx context.Context, m *session.SessionManager) error {
	_, err := m.EnableChromeTesting(ctx, true, []string{"--profile-requires-policy=true"}, []string{})
	return err
}
