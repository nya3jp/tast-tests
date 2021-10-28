// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/externaldata"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExtensionPolicy,
		Desc: "Check if extension policies can be applied",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
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

	// Start a Chrome instance with the test extension loaded.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.UnpackedExtension(extDir))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Extension policy needs to be hosted.
	eds, err := externaldata.NewServer(ctx)
	if err != nil {
		s.Fatal("Failed to create server: ", err)
	}
	defer eds.Stop(ctx)

	b, err := ioutil.ReadFile(s.DataPath("extension_policy/policy.json"))
	if err != nil {
		s.Fatal("Failed to load extension_policy.json: ", err)
	}
	pURL, pHash := eds.ServePolicyData(b)

	// Set the extension policy.
	pb := fakedms.NewPolicyBlob()
	if err := pb.AddExtensionPolicy(extID, pURL, pHash); err != nil {
		s.Fatal("Failed to set the extension policy: ", err)
	}
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to apply policies: ", err)
	}

	// Connect to the extension and read the set values.
	bgURL := chrome.ExtensionBackgroundPageURL(extID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to connect to background page at %v: %v", bgURL, err)
	}
	defer conn.Close()

	testing.Sleep(ctx, 1*time.Second)

	js := `new Promise((resolve, reject) => chrome.storage.managed.get(['SensitiveStringPolicy', 'SensitiveDictPolicy'], resolve))`

	var val json.RawMessage
	if err := conn.Eval(ctx, js, &val); err != nil {
		s.Fatal("Failed to read managed storage: ", err)
	}
	s.Log("Managed value: ", string(val))

	/*if val != "secret" {
		s.Error("Unexpected value read from managed storage: want %q; got %q", "secret", val)
	}*/
}
