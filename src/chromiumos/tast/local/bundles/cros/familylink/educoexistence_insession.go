// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EducoexistenceInsession,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if in-session EDU Coexistence flow is working",
		Contacts:     []string{"agawronska@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 5*time.Minute,
		VarDeps:      []string{"family.parentEmail", "family.parentPassword", "family.eduEmail", "family.eduPassword"},
		Params: []testing.Param{{
			Fixture: "familyLinkUnicornLogin",
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "familyLinkUnicornLoginWithLacros",
		}},
	})
}

func EducoexistenceInsession(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	parentUser := s.RequiredVar("family.parentEmail")
	parentPass := s.RequiredVar("family.parentPassword")
	eduUser := s.RequiredVar("family.eduEmail")
	eduPass := s.RequiredVar("family.eduPassword")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Launching the in-session Edu Coexistence flow")
	if err := familylink.AddEduSecondaryAccount(ctx, cr, tconn, parentUser, parentPass, eduUser, eduPass, true /*verifyEduSecondaryAddSuccess*/); err != nil {
		s.Fatal("Failed to complete the in-session Edu Coexistence flow: ", err)
	}

	// TODO(b/254131536): Check the EDU account is added as secondary to browser web page.
	// Now that the multi profile in Lacros is disabled for supervised users,
	// the experience in Ash and Lacros should be similar:
	// one browser profile with 2 accounts (supervised account as primary and EDU account as secondary).
}
