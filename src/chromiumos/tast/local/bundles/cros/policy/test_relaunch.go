// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TestRelaunch,
		Desc: "Behavior of AllowDinosaurEasterEgg policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromeLoggedIn",
	})
}

func TestRelaunch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	testexec.CommandContext(ctx, "update_engine_client", "--override_reboot_required").Run()

	// Put test code here.
	policyutil.OSSettingsPage(ctx, cr, "help")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	restart := nodewith.Name("Restart").Role(role.Button)

	ui.WaitUntilExists(restart)(ctx)
	ui.LeftClick(restart)(ctx)

	testing.Sleep(ctx, time.Second*5)

	s.Fatal("I would like a UI dump")
}
