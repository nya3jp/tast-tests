// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
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
	// Open admin desk template for view
	templateJSON, err := getJSONFileFromFilePath(s.DataPath("admin_desk_template.json"))

	eds, err := externaldata.NewServer(ctx)
	if err != nil {
		s.Fatal("Failed to create server: ", err)
	}
	defer eds.Stop(ctx)

	iurl, ihash := eds.ServePolicyData(templateJSON)

	for _, param := range []struct {
		name  string
		value *policy.PreconfiguredDeskTemplates // value is the value of the policy.
	}{
		{
			name:  "nonempty",
			value: &policy.PreconfiguredDeskTemplates{Val: &policy.PreconfiguredDeskTemplatesValue{Url: iurl, Hash: ihash}},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)
			ac := uiauto.New(tconn)
			testing.Sleep(ctx, 30*time.Second)

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}
			// Enters overview mode.
			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to set overview mode: ", err)
			}

			if err := ac.WaitForLocation(nodewith.Root())(ctx); err != nil {
				s.Fatal("Failed to wait for overview animation to be completed: ", err)
			}
			// Find the "Templates" button.
			templatesButton := nodewith.Name("Templates")
			desksTemplatesGridView := nodewith.ClassName("DesksTemplatesGridView")
			// Show admin desk template.
			if err := uiauto.Combine(
				"show the admin desk template",
				ac.LeftClick(templatesButton),
				// Wait for the desks templates grid shows up.
				ac.WaitUntilExists(desksTemplatesGridView),
			)(ctx); err != nil {
				s.Fatal("Failed to show saved desks templates: ", err)
			}

			// Find the the admin template.
			adminTemplate := nodewith.ClassName("DesksTemplatesItemView")
			newDeskMiniView :=
				nodewith.ClassName("DeskMiniView").Name(fmt.Sprintf("Desk: %s", "Desk 1"))

			// Check admin template icon is there

			if err := uiauto.Combine(
				"check that admin template icon is there",
				ac.Exists(nodewith.ClassName("ImageViewer")),
			)(ctx); err != nil {
				s.Fatal("Failed to find admin template icon")
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

// getJSONFileFromFilePath returns bytes of json file with the file path
func getJSONFileFromFilePath(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	byteValue, _ := ioutil.ReadAll(f)
	var jsonResult []byte
	json.Unmarshal([]byte(byteValue), &jsonResult)

	return jsonResult, nil
}
