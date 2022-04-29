// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/sysutil"
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
		Data:         []string{"extension_policy/background.js", "extension_policy/manifest.json", "extension_policy/schema.json"},
	})
}

const extensionPolicyDir = "extension_policy"

var extensionPolicyFiles = []string{"background.js", "manifest.json", "schema.json"}

func ExtensionPoliciesMGS(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Load extension as unpacked.
	extDir, err := ioutil.TempDir("", "policy_test_extension")
	if err != nil {
		s.Fatal("Failed to create temporary directory for test extension: ", err)
	}
	defer os.RemoveAll(extDir)

	if err := os.Chown(extDir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		s.Fatal("Failed to chown test extension dir: ", err)
	}

	for _, file := range extensionPolicyFiles {
		source := filepath.Join(extensionPolicyDir, file)
		target := filepath.Join(extDir, file)
		if err := fsutil.CopyFile(s.DataPath(source), target); err != nil {
			s.Fatalf("Failed to copy %q file to %q: %v", file, extDir, err)
		}

		if err := os.Chown(target, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
			s.Fatalf("Failed to chown %q: %v", file, err)
		}
	}

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}

	// Configure initial extension policy.
	pb := policy.NewBlob()
	pb.ExtensionPM = make(policy.BlobPolicyMap)
	pb.ExtensionPM[extID] = json.RawMessage(`{"VisibleStringPolicy": {"Value": "initialValue"}}`)

	// Start public session.
	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
		mgs.ExternalPolicyBlob(pb),
		mgs.ExtraChromeOptions(chrome.UnpackedExtension(extDir)),
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
	bgURL := chrome.ExtensionBackgroundPageURL(extID)
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
		VisibleStringPolicy string
	}

	var extensionPolicy ExtensionPolicy
	if err := json.Unmarshal(rawPolicy, &extensionPolicy); err != nil {
		s.Fatal("Failed to parse extension policies")
	}

	providedPolicy := ExtensionPolicy{VisibleStringPolicy: "initialValue"}
	if extensionPolicy != providedPolicy {
		s.Errorf("Invaid initial extension policy: got %v, expected %v", extensionPolicy, providedPolicy)
	}

	// Change the extension policy on the server.
	pb.ExtensionPM[extID] = json.RawMessage(`{"VisibleStringPolicy": {"Value": "updatedValue"}}`)
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

	providedPolicy = ExtensionPolicy{VisibleStringPolicy: "updatedValue"}
	if extensionPolicy != providedPolicy {
		s.Errorf("Invaid updated extension policy: got %v, expected %v", extensionPolicy, providedPolicy)
	}
}
