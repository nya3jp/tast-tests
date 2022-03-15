// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VariationSmoke,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"yjt@google.com", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
		Timeout:      5 * time.Minute,
		Data:         []string{"variation_seed.json"},
	})
}

// Constants for the path to CrOS' Local State file and the pref names for controlling the variations config.
const (
	localStatePath     = "/home/chronos/user/lacros/Local State"
	compressedSeedPref = "variations_compressed_seed"
	seedSignaturePref  = "variations_seed_signature"
)

// variationsSeedData represents a variations seed, which contains information about field trials to enable on the device.
type variationsSeedData struct {
	CompressedSeed string `json:"variations_compressed_seed"`
	SeedSignature  string `json:"variations_seed_signature"`
}

// Read Lacros Local State and store the info into a map
func readLocalStateFile() (map[string]interface{}, error) {
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
	return localStateMap, nil

}

// readVariationsSeed reads the current variations seed from the Local State file.
func readVariationsSeed(ctx context.Context, s *testing.State) (*variationsSeedData, error) {
	localStateMap, err := readLocalStateFile()
	if err != nil {
		return nil, errors.Errorf("Failed to read Loal State file")
	}

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

// injectVariationsSeed injects the given seed into Local State. The seed will be loaded and take effect on the next run of Lacros.
func injectSeedInLocalState(ctx context.Context, tconn *chrome.TestConn, seed *variationsSeedData) error {
	localStateMap, err := readLocalStateFile()
	if err != nil {
		return errors.Errorf("Failed to read Loal State file")
	}
	// inject the seed
	localStateMap[compressedSeedPref] = seed.CompressedSeed
	localStateMap[seedSignaturePref] = seed.SeedSignature
	jsonStr, err := json.Marshal(localStateMap)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal Local State after injecting the seed")
	}
	if err := ioutil.WriteFile(localStatePath, jsonStr, 0644); err != nil {
		return errors.Wrap(err, "failed to write Local State with injected seed")
	}
	return nil
}

func VariationSmoke(ctx context.Context, s *testing.State) {
	//extra args when launch lacros, to ensure variation service is enabled, these two args are required to set
	extraArgs := []string{"--fake-variations-channel=beta", "--variations-server-url=https://clients4.google.com/chrome-variations/seed"}

	// Prepare the test seed.
	testSeedFile, err := os.Open(s.DataPath("variation_seed.json"))
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
	func() {

		cr, err := lacros.LaunchWithExtraArgs(ctx, s.FixtValue().(lacrosfixt.FixtValue), extraArgs)
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer cr.Close(ctx)
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}
		if err := testing.Poll(ctx, func(context.Context) error {
			if _, err := readVariationsSeed(ctx, s); err != nil {
				return errors.Wrap(err, "production variations seed not yet fetched")
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
			s.Fatal("Production variations seed not fetched: ", err)
		}
		if err := injectSeedInLocalState(ctx, tconn, &testSeed); err != nil {
			s.Fatal("Failed to inject test seed: ", err)
		}
		// Ensure the seed was injected. There is no waiting time required because it will not download the seed.
		currentSeed, err := readVariationsSeed(ctx, s)
		if err != nil {
			s.Fatal("failed to read variations seed info")
		}
		if currentSeed.CompressedSeed != testSeed.CompressedSeed || currentSeed.SeedSignature != testSeed.SeedSignature {
			s.Fatal("Local State has not updated with the test seed")
		}

	}()

	// Restart lacros with the injected seed.
	cr, err := lacros.LaunchWithExtraArgs(ctx, s.FixtValue().(lacrosfixt.FixtValue), extraArgs)
	if err != nil {
		s.Fatal("Failed to launch lacros: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Verify that Lacros has downloaded and updated the variations seed. Poll to allow some time for downloading the seed.
	if err := testing.Poll(ctx, func(context.Context) error {
		currentSeed, err := readVariationsSeed(ctx, s)
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
		url     string
		text    string
		content string
	}
	tests := []tc{
		{
			//data:text/html,<h1 id="success">Success</h1>
			url:     "data:text/html,%3Ch1%20id%3D%22success%22%3ESuccess%3C%2Fh1%3E",
			text:    "Success",
			content: "<h1 id=\"success\">Success</h1>",
		},
		{
			// TODO(crbug.com/1234165): Make tests hermetic by using a test http server.
			url:     "https://chromium.org/",
			text:    "The Chromium Projects",
			content: "<h2>The Chromium Projects</h2>",
		},
	}

	uiauto.New(tconn)
	for _, t := range tests {
		c, err := cr.NewConn(ctx, t.url)
		if err != nil {
			s.Fatalf("Failed to open Lacros with URL %v: %v", t.url, err)
		}
		defer c.Close()
		pageContent, err := c.PageContent(ctx)
		if err != nil {
			s.Fatal("Failed to obtain the page content")
		}

		if !strings.Contains(pageContent, t.content) {
			s.Fatalf("Failed to find text %q on page %q with correct format", t.text, t.url)
		}
	}

}
