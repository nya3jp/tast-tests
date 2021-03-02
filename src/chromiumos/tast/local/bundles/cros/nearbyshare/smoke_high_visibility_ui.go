// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"

	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SmokeHighVisibilityUI,
		Desc: "Checks that Nearby Share high-visibility receiving can be initiated from Quick Settings",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "nearbyShareDataUsageOfflineAllContactsTestUserNoAndroid",
	})
}

// SmokeHighVisibilityUI tests that we can open the receiving UI surface from Quick Settings.
func SmokeHighVisibilityUI(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*nearbyshare.FixtData).TestConn
	deviceName := s.FixtValue().(*nearbyshare.FixtData).CrOSDeviceName
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	if err := nearbyshare.StartHighVisibilityMode(ctx, tconn, deviceName); err != nil {
		s.Fatal("Failed to enable Nearby Share's high visibility mode: ", err)
	}
}
