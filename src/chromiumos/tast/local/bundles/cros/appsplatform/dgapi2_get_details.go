// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/playbilling/dgapi2"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dgapi2GetDetails,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify DGAPI2 test app returns expected details",
		Contacts: []string{
			"jshikaram@chromium.org",
			"ashpakov@google.com", // until Sept 2022
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "playBillingDgapi2Fixture",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 5 * time.Minute,
	})
}

// Dgapi2GetDetails Checks DGAPI2 test app returns details.
func Dgapi2GetDetails(ctx context.Context, s *testing.State) {
	p := s.FixtValue().(*dgapi2.FixtDgapiData)
	cr := p.Chrome
	testApp := p.TestApp

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "Dgapi2GetDetails")

	if err := testApp.VerifyDetailsLogs(ctx); err != nil {
		s.Fatal("Failed to verify logs: ", err)
	}
}
