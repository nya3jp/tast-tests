// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtensionPolicy,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check if extension policies can be applied",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Timeout:      2 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMS,
		Data:         []string{"extension_policy/policy.json", "extension_policy/background.js", "extension_policy/manifest.json", "extension_policy/schema.json"},
	})
}

const extensionPolicyDir = "extension_policy"

var extensionPolicyFiles = []string{"background.js", "manifest.json", "schema.json"}

func ExtensionPolicy(ctx context.Context, s *testing.State) {
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

	b, err := ioutil.ReadFile(s.DataPath("extension_policy/policy.json"))
	if err != nil {
		s.Fatal("Failed to load extension_policy.json: ", err)
	}

	// Set the extension policy.
	pb := policy.NewBlob()
	if err := pb.AddExtensionPolicy(extID, json.RawMessage(b)); err != nil {
		s.Fatal("Failed to set the extension policy: ", err)
	}
	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies: ", err)
	}

	// Start a Chrome instance with the test extension loaded.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.UnpackedExtension(extDir),
		chrome.ExtraArgs("--vmodule=*=1"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get a test api connection: ", err)
	}

	policies, err := policyutil.PoliciesFromDUT(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to read policies from DUT: ", err)
	}

	type ExtensionPolicies struct {
		// Not handling all fields in the schema.
		SensitiveStringPolicy string `json:"SensitiveStringPolicy"`
		VisibleStringPolicy   string `json:"VisibleStringPolicy"`
	}

	providedPolicy := ExtensionPolicies{
		SensitiveStringPolicy: "secret",
		VisibleStringPolicy:   "notsecret",
	}

	var receivedPolicy ExtensionPolicies
	if err := json.Unmarshal(
		policies.Extension[extID]["VisibleStringPolicy"].ValueJSON,
		&receivedPolicy.VisibleStringPolicy); err != nil {
		s.Fatal("Failed to parse received policies")
	}
	if err := json.Unmarshal(
		policies.Extension[extID]["SensitiveStringPolicy"].ValueJSON,
		&receivedPolicy.SensitiveStringPolicy); err != nil {
		s.Fatal("Failed to parse received policies")
	}

	expectedPolicy := providedPolicy
	expectedPolicy.SensitiveStringPolicy = "********" // Sensitive strings are hidden.

	if receivedPolicy != expectedPolicy {
		s.Errorf("Extension policy dump missmatch: got %v; want %v", receivedPolicy, expectedPolicy)
	}

	// Connect to the extension and read the set values.
	bgURL := chrome.ExtensionBackgroundPageURL(extID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to connect to background page at %v: %v", bgURL, err)
	}
	defer conn.Close()

	if err := chrome.AddTastLibrary(ctx, conn); err != nil {
		s.Fatal("Failed to add Tast library to PWA: ", err)
	}

	var val json.RawMessage
	if err := conn.Call(ctx, &val,
		`tast.promisify(tast.bind(chrome.storage.managed, "get"))`,
		[]string{"SensitiveStringPolicy", "VisibleStringPolicy"}); err != nil {

		s.Fatal("Failed to read managed storage: ", err)
	}

	var extensionPolicy ExtensionPolicies
	if err := json.Unmarshal(val, &extensionPolicy); err != nil {
		s.Fatal("Failed to parse extension policies")
	}

	if extensionPolicy != providedPolicy {
		s.Errorf("Extension policy missmatch: got %v; want %v", extensionPolicy, providedPolicy)
	}
}
