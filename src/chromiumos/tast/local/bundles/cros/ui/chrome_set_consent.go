// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeSetConsent,
		Desc:         "Demostrates issues with SetConsent",
		Contacts:     []string{"chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Data: []string{
			crash.MockMetricsOnPolicyFile,
			crash.MockMetricsOwnerKeyFile,
		},
	})
}

func testChromeSetConsent(ctx context.Context, s *testing.State) {
	// KeepState is necessary to keep the tast framework from erasing the consent files.
	cr, err := chrome.New(ctx, chrome.KeepState())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
}

func ChromeSetConsent(ctx context.Context, s *testing.State) {
	crash.RunCrashTest(ctx, s, testChromeSetConsent)
}
