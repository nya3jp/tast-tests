// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GuestModeExtension,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check Tast extension can be loaded in Guest mode",
		Contacts:     []string{"benreich@chromium.org", "chromeos-engprod-syd@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromeLoggedInGuest",
	})
}

func GuestModeExtension(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	// Attempt to open the Test API connection.
	if _, err := cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create Test API Connection: ", err)
	}
}
