// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeVariationsSmoke,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Chrome doesn't crash and basic web content rendering is functional when loading a given variations seed",
		Contacts: []string{
			"kyleshima@chromium.org", // Test author
			"chromeos-sw-engprod@google.com",
			// Variations owners. Refer to //base/metrics/OWNERS for the most up-to-date contacts.
			"isherman@chromium.org",
			"asvitkine@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"variations_seed_beta_chromeos.json"},
		Vars:         []string{"fakeVariationsChannel", "useSeedOnDisk"},
		Timeout:      5 * time.Minute,
	})
}

// Constants for the path to CrOS' Local State file and the pref names for controlling the variations config.
const (
	localStatePath     = "/home/chronos/Local State"
	compressedSeedPref = "variations_compressed_seed"
	seedSignaturePref  = "variations_seed_signature"
)

// variationsSeedData represents a variations seed, which contains information about field trials to enable on the device.
type variationsSeedData struct {
	CompressedSeed string `json:"variations_compressed_seed"`
	SeedSignature  string `json:"variations_seed_signature"`
}

// readVariationsSeed reads the current variations seed from the Local State file.
func readVariationsSeed(ctx context.Context) (*variationsSeedData, error) {
	localStateFile, err := os.Open(localStatePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open Local State file")
	}
	defer localStateFile.Close()

	var localState interface{}
	b, err := ioutil.ReadAll(localStateFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read Local State file contents")
	}
	if err := json.Unmarshal(b, &localState); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal Local State")
	}
	localStateMap := localState.(map[string]interface{})

	seed, ok := localStateMap[compressedSeedPref].(string)
	if !ok {
		return nil, errors.Errorf("%v field not found in Local State", compressedSeedPref)
	}
	signature, ok := localStateMap[seedSignaturePref].(string)
	if !ok {
		return nil, errors.Errorf("%v field not found in Local State", seedSignaturePref)
	}
	return &variationsSeedData{CompressedSeed: seed, SeedSignature: signature}, nil
}

// injectVariationsSeed injects the given seed into Local State. The seed will be loaded and take effect on the next run of Chrome (i.e. next user session).
func injectVariationsSeed(ctx context.Context, tconn *chrome.TestConn, seed *variationsSeedData) error {
	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setWhitelistedPref)", seedSignaturePref, seed.SeedSignature); err != nil {
		return errors.Wrapf(err, "failed to set %v", seedSignaturePref)
	}
	if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setWhitelistedPref)", compressedSeedPref, seed.CompressedSeed); err != nil {
		return errors.Wrapf(err, "failed to set %v", compressedSeedPref)
	}
	return nil
}

// ChromeVariationsSmoke tests that Chrome doesn't crash and basic web content rendering is functional when loading a given variations seed.
func ChromeVariationsSmoke(ctx context.Context, s *testing.State) {
	// Prepare the test seed. By default, use the seed provided through the test data.
	// If the useSeedOnDisk runtime var is specified as true, look for a test seed at /opt/google/chrome/variations_seed.txt instead.
	// This file will be present when the test runs as part of the variations smoke test suite in Chromium CI.
	seedPath := s.DataPath("variations_seed_beta_chromeos.json")
	if val, ok := s.Var("useSeedOnDisk"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert useSeedOnDisk var to bool: ", err)
		}
		if b {
			seedPath = "/opt/google/chrome/variations_seed.txt"
		}
	}

	s.Log("Using test seed from ", seedPath)
	testSeedFile, err := os.Open(seedPath)
	if err != nil {
		s.Fatal("Failed to open test seed file: ", err)
	}
	defer testSeedFile.Close()
	b, err := ioutil.ReadAll(testSeedFile)
	if err != nil {
		s.Fatal("Failed to read Local State file contents: ", err)
	}
	var testSeed variationsSeedData
	if err := json.Unmarshal(b, &testSeed); err != nil {
		s.Fatal("Failed to unmarshal test seed: ", err)
	}

	// Log in, verify a production seed was fetched, and inject the test seed.
	// The injected test seed will take effect on the next start of Chrome.
	func() {
		// Chrome OS test images always have "unknown" browser channel since they are on testimage-channel.
		// Variations configs are typically not served to unknown channels, so we need to specify
		// --fake-variations-channel to successfully fetch and apply variations configs.
		// We will use beta as the default channel (corresponding to the hardcoded seed in data/),
		// unless a different channel is specified in the runtime vars.
		// Also, specify the variations server explicitly, otherwise variations configs won't be fetched
		// on builds that are not Chrome-branded.
		channel := "beta"
		if val, ok := s.Var("fakeVariationsChannel"); ok {
			s.Log("Setting fake-variations-channel to ", val)
			channel = val
		}
		cr, err := chrome.New(ctx, chrome.ExtraArgs(
			"--fake-variations-channel="+channel,
			"--variations-server-url=https://clients4.google.com/chrome-variations/seed",
		))
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}
		if err := testing.Poll(ctx, func(context.Context) error {
			if _, err := readVariationsSeed(ctx); err != nil {
				return errors.Wrap(err, "production variations seed not yet fetched")
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
			s.Fatal("Production variations seed not fetched: ", err)
		}
		if err := injectVariationsSeed(ctx, tconn, &testSeed); err != nil {
			s.Fatal("Failed to inject test seed: ", err)
		}

		// Ensure the seed was injected. It can take some time for Local State to update with the injected seed.
		if err := testing.Poll(ctx, func(context.Context) error {
			currentSeed, err := readVariationsSeed(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to read variations seed info")
			}
			if currentSeed.CompressedSeed != testSeed.CompressedSeed || currentSeed.SeedSignature != testSeed.SeedSignature {
				return errors.New("Local State has not updated with the test seed")
			}
			return nil
		}, nil); err != nil {
			s.Fatal("The test seed was not injected: ", err)
		}
	}()

	// Restart Chrome with the test seed injected.
	cr, err := chrome.New(ctx, chrome.KeepState(), chrome.ExtraArgs(
		"--fake-variations-channel=beta",
		"--variations-server-url=https://clients4.google.com/chrome-variations/seed",
	))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Verify that Chrome has downloaded and updated the variations seed. Poll to allow some time for downloading the seed.
	if err := testing.Poll(ctx, func(context.Context) error {
		currentSeed, err := readVariationsSeed(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read variations seed info")
		}
		if currentSeed.CompressedSeed == testSeed.CompressedSeed || currentSeed.SeedSignature == testSeed.SeedSignature {
			return errors.New("chrome did not update the variations seed")
		}
		return nil
	}, nil); err != nil {
		s.Fatal("The test seed was not injected: ", err)
	}

	// Navigate to some pages in Chrome and verify that web elements are rendered correctly.
	type tc struct {
		url    string
		text   string
		finder *nodewith.Finder
	}
	tests := []tc{
		{
			//data:text/html,<h1 id="success">Success</h1>
			url:    "data:text/html,%3Ch1%20id%3D%22success%22%3ESuccess%3C%2Fh1%3E",
			text:   "Success",
			finder: nodewith.Name("Success").Role(role.Heading),
		},
		{
			// TODO(crbug.com/1234165): Make tests hermetic by using a test http server.
			url:    "https://chromium.org/",
			text:   "The Chromium Projects",
			finder: nodewith.Name("The Chromium Projects").Role(role.Heading),
		},
	}
	ui := uiauto.New(tconn)
	for _, t := range tests {
		c, err := cr.NewConn(ctx, t.url)
		if err != nil {
			s.Fatalf("Failed to open Chrome browser with URL %v: %v", t.url, err)
		}
		defer c.Close()

		if err := ui.WaitUntilExists(t.finder)(ctx); err != nil {
			s.Fatalf("Failed to find text %q on page %q: %v", t.text, t.url, err)
		}
	}
}
