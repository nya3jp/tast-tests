// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Autostart,
		Desc: "Checks that Kiosk configuration starts when set to autologin",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alt-modalities-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "kioskLoggedIn",
	})
}

// Autostart TODO: Once a test with more checks will be developed this test can
//  be safely removed.
func Autostart(ctx context.Context, s *testing.State) {
}
