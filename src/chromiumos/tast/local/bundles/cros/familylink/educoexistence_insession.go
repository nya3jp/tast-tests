// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks if in-session EDU Coexistence flow is working",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 5*time.Minute,
		VarDeps:      []string{"unicorn.parentUser", "unicorn.parentPassword", "unicorn.parentFirstName", "unicorn.parentLastName", "edu.user", "edu.password"},
		Fixture:      "familyLinkUnicornLogin",
	})
}

func EducoexistenceInsession(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	cr := s.FixtValue().(*familylink.FixtData).Chrome

	parentUser := s.RequiredVar("unicorn.parentUser")
	parentPass := s.RequiredVar("unicorn.parentPassword")
	parentFirstName := s.RequiredVar("unicorn.parentFirstName")
	parentLastName := s.RequiredVar("unicorn.parentLastName")
	eduUser := s.RequiredVar("edu.user")
	eduPass := s.RequiredVar("edu.password")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Launching the in-session Edu Coexistence flow")
	if err := familylink.AddEduSecondaryAccountWithMultipleParents(ctx, cr, tconn, parentFirstName, parentLastName,
		parentUser, parentPass, eduUser, eduPass, true /*verifyEduSecondaryAddSuccess*/); err != nil {
		s.Fatal("Failed to complete the in-session Edu Coexistence flow: ", err)
	}
}
