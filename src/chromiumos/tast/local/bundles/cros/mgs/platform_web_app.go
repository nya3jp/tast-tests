// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/testing"
)

const fileName = "chrome___dino_.pdf"

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlatformWebApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that Platform Web Apps (PWA) are working in a managed guest session by trying to install and start Google News",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func PlatformWebApp(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
	)
	if err != nil {
		s.Fatal("Failed to start MGS: ", err)
	}
	defer mgs.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	id, err := apps.InstallPWAForURL(ctx, cr, "https://news.google.com", time.Second*30)
	if err != nil {
		s.Fatal("Failed to install google news PWA: ", err)
	}
	// No need to uninstall as the app should be gone after session is closed.

	if err := apps.Launch(ctx, tconn, id); err != nil {
		s.Fatal("Failed to launch google news PWA: ", err)
	}

}
