// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FakeEnrollmentRealGAIA,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that real GAIA account can be used along with fake enrollment",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		// Disabled due to <1% pass rate over 30 days. See b/246818601
		//Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		Vars: []string{
			"policy.FakeEnrollmentRealGAIA.accountPool",
		},
	})
}

// FakeEnrollmentRealGAIA tests that real GAIA account can be used along with fake enrollment.
func FakeEnrollmentRealGAIA(ctx context.Context, s *testing.State) {
	fdms, ok := s.FixtValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	gaiaCreds, err := chrome.PickRandomCreds(s.RequiredVar("policy.FakeEnrollmentRealGAIA.accountPool"))
	if err != nil {
		s.Fatal("Failed to parse managed user creds: ", err)
	}

	username := gaiaCreds.User
	password := gaiaCreds.Pass

	pb := policy.NewBlob()
	pb.PolicyUser = username
	pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	pb.UserAffiliationIds = []string{"default_affiliation_id"}

	// We have to update fake DMS policy user and affiliation IDs before starting Chrome.
	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policy blob before starting Chrome: ", err)
	}

	cr, err := chrome.New(ctx,
		chrome.KeepEnrollment(),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout))
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(ctx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Set some policy.
	ps := []policy.Policy{&policy.DefaultSearchProviderEnabled{Val: true}}
	pb.AddPolicies(ps)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	if err := policyutil.Verify(ctx, tconn, ps); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	conn, err := cr.NewConn(ctx, "chrome://policy")
	if err != nil {
		s.Fatal("Failed to create connection to policy page: ", err)
	}
	defer conn.Close()

	if err := conn.Eval(ctx, `document.getElementById("export-policies").click()`, nil); err != nil {
		s.Fatal("Failed to click export to JSON button: ", err)
	}

	// Clear Downloads directory.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	policiesPath := filepath.Join(downloadsPath, "policies.json")

	if _, err := os.Stat(policiesPath); err == nil {
		if err := os.Remove(policiesPath); err != nil {
			s.Fatal("Failed to remove policies.json: ", err)
		}
	}

	ui := uiauto.New(tconn)
	saveButton := nodewith.Name("Save").Role(role.Button)
	if err := uiauto.Combine("click Save",
		ui.WithTimeout(10*time.Second).WaitUntilExists(saveButton),
		ui.LeftClick(saveButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click save file button: ", err)
	}

	// Wait until policies.json will be created.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(policiesPath); errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to wait policies.json file to exist: ", err)
	}

	content, err := ioutil.ReadFile(policiesPath)
	if err != nil {
		s.Fatal("Failed to read policies.json: ", err)
	}

	if !strings.Contains(string(content), `"isAffiliated": true`) {
		s.Fatal("policies.json does not confirm that the user is affiliated")
	}
}
