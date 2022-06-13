// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdminTemplatesLaunch,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks admin templates can be launched",
		Contacts: []string{
			"zhumatthew@google.com",
			"chromeos-wmp@google.com",
			"cros-commercial-productivity-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Fixture:      fixture.ChromeAdminDeskTemplatesLoggedIn,
		Data:         []string{"admin_desk_template.json"},
	})
}

func AdminTemplatesLaunch(ctx context.Context, s *testing.State) {
	const adminDeskTemplateName = "Test Admin template for default user"

	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)
	// Open admin desk template for view.
	templateJSON, err := getJSONFileFromFilePath(s.DataPath("admin_desk_template.json"))

	eds, err := externaldata.NewServer(ctx)
	if err != nil {
		s.Fatal("Failed to create server: ", err)
	}
	defer eds.Stop(ctx)

	iurl, ihash := eds.ServePolicyData(templateJSON)

	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	policiesToServe := []policy.Policy{
		&policy.PreconfiguredDeskTemplates{Val: &policy.PreconfiguredDeskTemplatesValue{Url: iurl, Hash: ihash}},
		&policy.DeskTemplatesEnabled{Val: true},
	}
	const subtestName = "nonempty"
	{
		s.Run(ctx, subtestName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+subtestName)
			ac := uiauto.New(tconn)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies for desk templates enabled policy.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, policiesToServe); err != nil {
				s.Fatal("Failed to update desk templates enabled policies: ", err)
			}

			// Wait 10 seconds for the templates icon to show up. There is some lag
			// for when the policy is pushed and when the templates icon will appear
			// in the overview bar.
			testing.Sleep(ctx, 10*time.Second)

			// Enters overview mode.
			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to set overview mode: ", err)
			}

			if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
				s.Fatal("Failed to wait for overview animation to be completed: ", err)
			}
			// Find the "Library" button.
			templatesButton := nodewith.Name("Library")
			desksTemplatesGridView := nodewith.ClassName("SavedDeskLibraryView")
			// Show admin desk template.
			if err := uiauto.Combine(
				"show the admin desk template",
				ac.LeftClick(templatesButton),
				// Wait for the desk templates grid shows up.
				ac.WaitUntilExists(desksTemplatesGridView),
			)(ctx); err != nil {
				s.Fatal("Failed to show admin desk templates: ", err)
			}

			// Find the the admin template.
			adminTemplate := nodewith.ClassName("SavedDeskItemView")
			newDeskMiniView :=
				nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", adminDeskTemplateName))

			// Check that the admin template is there.

			if err := uiauto.Combine(
				"check that it is an admin template",
				ac.Exists(nodewith.ClassName("SavedDeskNameView").Name(adminDeskTemplateName)),
				ac.Exists(nodewith.Name("Shared by your administrator")),
			)(ctx); err != nil {
				s.Fatal("Failed to find an admin desk template")
			}

			// Launch the admin template.
			if err := uiauto.Combine(
				"launch the admin template",
				ac.LeftClick(adminTemplate),
				// Wait for the new desk to appear.
				ac.WaitUntilExists(newDeskMiniView),
			)(ctx); err != nil {
				s.Fatal("Failed to launch a admin template: ", err)
			}
			// Exits overview mode.
			if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
				s.Fatal("Failed to exit overview mode: ", err)
			}

			// Verifies that there are the app windows.
			ws, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get all open windows: ", err)
			}

			if len(ws) != 2 {
				s.Fatalf("Got %v window(s), should have %v windows", len(ws), 2)
			}
		})
	}
}

// getJSONFileFromFilePath returns bytes of json file with the file path.
func getJSONFileFromFilePath(filePath string) ([]byte, error) {
	byteValue, _ := ioutil.ReadFile(filePath)
	var jsonResult interface{}
	json.Unmarshal(byteValue, &jsonResult)
	return json.Marshal(jsonResult)
}
