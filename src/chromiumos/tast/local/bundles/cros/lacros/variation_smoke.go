// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/local/variations"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VariationSmoke,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that Lacros doesn't crash and basic web content rendering is functional when loading a given variations seed",
		Contacts:     []string{"yjt@google.com", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosVariationEnabled",
		Timeout:      5 * time.Minute,
		Data:         []string{"variation_seed.json", "variation_test_index.html", "logo_chrome_color_1x_web_32dp.png"},
		Vars:         append([]string{"fakeVariationsChannel", "useSeedOnDisk"}, screenshot.ScreenDiffVars...),
	})
}

// Constants for the path to CrOS' Lacros Local State file and the pref names for controlling the variations config.
const (
	localStatePath  = lacros.UserDataDir + "/Local State"
	lacrosDeployDir = "/usr/local/lacros-chrome"
)

// readVariationsSeed reads the current variations seed from the Local State file.
func readVariationsSeed(ctx context.Context) (*variations.SeedData, error) {
	seedVal, err := localstate.UnmarshalPref(browser.TypeLacros, variations.CompressedSeedPref)
	if err != nil {
		return nil, errors.Errorf("%v field not found in Local State", variations.CompressedSeedPref)
	}
	seed, ok := seedVal.(string)
	if !ok {
		return nil, errors.Errorf("%v field has an unexpected value type in Local State", variations.CompressedSeedPref)
	}
	signatureVal, err := localstate.UnmarshalPref(browser.TypeLacros, variations.SeedSignaturePref)
	if err != nil {
		return nil, errors.Errorf("%v field not found in Local State", variations.SeedSignaturePref)
	}
	signature, ok := signatureVal.(string)
	if !ok {
		return nil, errors.Errorf("%v field has an unexpected value type in Local State", variations.SeedSignaturePref)
	}
	return &variations.SeedData{CompressedSeed: seed, SeedSignature: signature}, nil
}

// injectSeedInLocalState injects the given seed into Local State. The seed will be loaded and take effect on the next run of Lacros.
func injectSeedInLocalState(ctx context.Context, seed *variations.SeedData) error {
	if err := localstate.MarshalPref(browser.TypeLacros, variations.CompressedSeedPref, seed.CompressedSeed); err != nil {
		return errors.Wrapf(err, "failed to write Local State with %s", variations.CompressedSeedPref)
	}
	if err := localstate.MarshalPref(browser.TypeLacros, variations.SeedSignaturePref, seed.SeedSignature); err != nil {
		return errors.Wrapf(err, "failed to write Local State with %s", variations.SeedSignaturePref)
	}
	return nil
}

func VariationSmoke(ctx context.Context, s *testing.State) {
	// Prepare the test seed.
	seedPath := s.DataPath("variation_seed.json")
	// If useSeedOnDisk is set to True, the test will use the seed on the device instead of
	// test directory cros/lacros/data/variation_seed.json.
	if val, ok := s.Var("useSeedOnDisk"); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert useSeedOnDisk var to bool: ", err)
		}
		if b {
			seedPath = filepath.Join(lacrosDeployDir, "variations_seed.txt")
		}
	}
	testSeedFile, err := os.Open(seedPath)
	if err != nil {
		s.Fatal("Failed to open test seed file: ", err)
	}
	defer testSeedFile.Close()
	b, err := ioutil.ReadAll(testSeedFile)
	if err != nil {
		s.Fatal("Failed to read Local State file contents: ", err)
	}
	var testSeed variations.SeedData
	if err := json.Unmarshal(b, &testSeed); err != nil {
		s.Fatal("Failed to unmarshal test seed: ", err)
	}
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	uda := uidetection.NewDefault(tconn)
	func() {
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		// Close the lacros, close ash app first
		defer ash.WaitForAppClosed(ctx, tconn, apps.Lacros.ID)
		defer l.Close(ctx)

		if err := testing.Poll(ctx, func(context.Context) error {
			if _, err := readVariationsSeed(ctx); err != nil {
				return errors.Wrap(err, "production variations seed not yet fetched")
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
			s.Fatal("Production variations seed not fetched: ", err)
		}
		if err := injectSeedInLocalState(ctx, &testSeed); err != nil {
			s.Fatal("Failed to inject test seed: ", err)
		}
		// Ensure the seed was injected. There is no waiting time required because it will not download the seed.
		currentSeed, err := readVariationsSeed(ctx)
		if err != nil {
			s.Fatal("Failed to read variations seed info")
		}
		if currentSeed.CompressedSeed != testSeed.CompressedSeed || currentSeed.SeedSignature != testSeed.SeedSignature {
			s.Fatal("Local State has not updated with the test seed")
		}
	}()
	func() {
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		// Close the lacros, close ash app first
		defer ash.WaitForAppClosed(ctx, tconn, apps.Lacros.ID)
		defer l.Close(ctx)

		// Navigate to some pages in Chrome and verify that web elements are rendered correctly.
		type tc struct {
			url           string
			text          string
			content       string
			skiaGoldImage string
		}
		// Use a local http server to reduce dependencies on the network and external webpage contents.
		server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
		defer server.Close()
		tests := []tc{
			{
				//data:text/html,<h1 id="success">Success</h1>
				url:     "data:text/html,%3Ch1%20id%3D%22success%22%3ESuccess%3C%2Fh1%3E",
				text:    "Success",
				content: "<h1 id=\"success\">Success</h1>",
			},
			{
				url:           filepath.Join(server.URL, "variation_test_index.html"),
				text:          "The Chromium Projects",
				content:       "<h2><a href=\"https://www.chromium.org/\" dir=\"ltr\" id=\"sites-chrome-userheader-title\">The Chromium Projects</a></h2>",
				skiaGoldImage: "finch_smoke_render_chromium_org_html",
			},
		}

		for _, t := range tests {
			c, err := l.NewConn(ctx, t.url)
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
			func() {
				if len(t.skiaGoldImage) > 0 {
					d, err := screenshot.NewDifferFromChrome(ctx, s, cr, screenshot.Config{DefaultOptions: screenshot.Options{WindowWidthDP: 652, WindowHeightDP: 484, WindowBorderWidthDP: 100}})
					if err != nil {
						s.Fatal("Failed to start screen differ: ", err)
					}
					defer d.DieOnFailedDiffs()
					if err := uiauto.Combine("create finch smoke screenshot",
						uda.WaitUntilExists(uidetection.Word("Home")),
						d.DiffWindow(ctx, "finch_smoke", screenshot.Retries(2), screenshot.RetryInterval(time.Millisecond*600)))(ctx); err != nil {
						s.Error("Failed the skia gold diff: ", err)
					}
				}
			}()
		}

		// Verify that Lacros has downloaded and updated the variations seed. Poll to allow some time for downloading the seed.
		if err := testing.Poll(ctx, func(context.Context) error {
			currentSeed, err := readVariationsSeed(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to read variations seed info")
			}
			if currentSeed.CompressedSeed == testSeed.CompressedSeed || currentSeed.SeedSignature == testSeed.SeedSignature {
				return errors.New("Lacros did not update the variations seed")
			}
			return nil
		}, nil); err != nil {
			s.Fatal("Failed to download the seed: ", err)
		}
	}()
}
