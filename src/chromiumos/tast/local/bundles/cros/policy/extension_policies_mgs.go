// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtensionPoliciesMGS,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify that extension policies reach the app on Managed Guest Session",
		Contacts: []string{
			"sergiyb@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Timeout:      4 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

const (
	enterpriseTestExtensionID   = "dbbinhebhbmlbjnjpeiledcefofbelcl"
	enterpriseTestExtensionName = "Enterprise Verified Access Test Bed"
	extensionVersion            = "3.1.28"
	defaultProxyServerURL       = "https://test-proxy-server-1.example.com/"
	updatedProxyServerURL       = "https://test-proxy-server-2.example.com/"
	extensionPolicyTemplate     = `{"ProxyUrl":{"Value":"%s"}}`
)

func ExtensionPoliciesMGS(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	pb := policy.NewBlob()
	pb.ExtensionPM = make(policy.BlobPolicyMap)
	pb.ExtensionPM[enterpriseTestExtensionID] = json.RawMessage(
		fmt.Sprintf(extensionPolicyTemplate, defaultProxyServerURL))

	// Start public session.
	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
		mgs.ExternalPolicyBlob(pb),
		// TODO(b/231708149): Start a local fake extension host and use update
		// URL to have Chrome fetch extension from there instead of talking
		// directly to the Chrome Web Store.
		mgs.AddPublicAccountPolicies(mgs.MgsAccountID, []policy.Policy{
			&policy.ExtensionInstallForcelist{Val: []string{enterpriseTestExtensionID}},
		}),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome on Signin screen with default MGS account: ", err)
	}
	defer func() {
		if err := mgs.Close(ctx); err != nil {
			s.Fatal("Failed close MGS: ", err)
		}
	}()
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Connect to the extension.
	// TODO: This fails to find the background page. Even if opened manually via
	// NewConn, it fails to connect to target. Occasionally, Chrome would fail
	// to login into the MGS completed.  Something's clearly broken here, so
	// needs fixing.
	bgURL := chrome.ExtensionBackgroundPageURL(enterpriseTestExtensionID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to connect to the background page at %v: %v", bgURL, err)
	}
	defer conn.Close()

	if err := chrome.AddTastLibrary(ctx, conn); err != nil {
		s.Fatal("Failed to add Tast library to PWA: ", err)
	}

	// Read the extension policy and check that it is valid.
	var rawPolicy json.RawMessage
	if err := conn.Call(ctx, &rawPolicy,
		`tast.promisify(tast.bind(chrome.storage.managed, "get"))`,
		[]string{"ProxyUrl"}); err != nil {
		s.Fatal("Failed to read managed storage: ", err)
	}

	type ExtensionPolicy struct {
		// Not handling all fields in the schema.
		ProxyURL string `json:"ProxyUrl"`
	}

	var extensionPolicy ExtensionPolicy
	if err := json.Unmarshal(rawPolicy, &extensionPolicy); err != nil {
		s.Fatal("Failed to parse extension policies")
	}

	providedPolicy := ExtensionPolicy{ProxyURL: defaultProxyServerURL}
	if extensionPolicy != providedPolicy {
		s.Errorf("Invaid initial extension policy: got %v, expected %v", extensionPolicy, providedPolicy)
	}

	// Change the extension policy on the server.
	pb.ExtensionPM[enterpriseTestExtensionID] = json.RawMessage(
		fmt.Sprintf(extensionPolicyTemplate, updatedProxyServerURL))
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh policies: ", err)
	}

	// Read extension policy again and check that it has been updated.
	if err := conn.Call(ctx, &rawPolicy,
		`tast.promisify(tast.bind(chrome.storage.managed, "get"))`,
		[]string{"ProxyUrl"}); err != nil {
		s.Fatal("Failed to read managed storage: ", err)
	}

	if err := json.Unmarshal(rawPolicy, &extensionPolicy); err != nil {
		s.Fatal("Failed to parse extension policies")
	}

	providedPolicy = ExtensionPolicy{ProxyURL: updatedProxyServerURL}
	if extensionPolicy != providedPolicy {
		s.Errorf("Invaid updated extension policy: got %v, expected %v", extensionPolicy, providedPolicy)
	}
}
