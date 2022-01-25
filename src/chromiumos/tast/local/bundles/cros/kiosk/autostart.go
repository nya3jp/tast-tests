// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Autostart,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Kiosk configuration starts when set to autologin",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"irfedorova@google.com",  // Lacros test autor
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: fixture.KioskLoggedInAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.KioskLoggedInLacros,
		}},
	})
}

// Autostart TODO: Once a test with more checks will be developed this test can
// be safely removed.
func Autostart(ctx context.Context, s *testing.State) {
}
