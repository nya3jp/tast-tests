// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/variations"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeVariationsSmoke,
		LacrosStatus: testing.LacrosVariantUnneeded, //variation_smoke.go is a test just for lacros
		Desc:         "Checks that Chrome doesn't crash and basic web content rendering is functional when loading a given variations seed",
		Contacts: []string{
			"kyleshima@chromium.org", // Test author
			"chromeos-sw-engprod@google.com",
			// Variations owners. Refer to //base/metrics/OWNERS for the most up-to-date contacts.
			"dalerogerson@google.com",
			"nishantj@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data: []string{
			"variations_seed_beta_chromeos.json",
			"variations_test_index.html",
			"logo_chrome_color_1x_web_32dp.png",
		},
		Vars:    append([]string{"fakeVariationsChannel", "useSeedOnDisk"}, screenshot.ScreenDiffVars...),
		Timeout: 5 * time.Minute,
	})
}

// readVariationsSeed reads the current variations seed from the Local State file.
func readVariationsSeed(ctx context.Context) (*variations.SeedData, error) {
	var seed variations.SeedData
	if err := localstate.Unmarshal(browser.TypeAsh, &seed); err != nil {
		return nil, err
	}
	return &seed, nil
}

// injectVariationsSeed injects the given seed into Local State. The seed will be loaded and take effect on the next run of Chrome (i.e. next user session).
func injectVariationsSeed(ctx context.Context, tconn *chrome.TestConn, seed *variations.SeedData) error {
	// The call to chrome.autotestPrivate.setAllowedPref can sometimes fail if called immediately after Chrome starts up.
	// To work around this, keep trying to inject the test seed until it sticks.
	if err := testing.Poll(ctx, func(context.Context) error {
		currentSeed, err := readVariationsSeed(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read variations seed info")
		}
		if currentSeed.CompressedSeed != seed.CompressedSeed || currentSeed.SeedSignature != seed.SeedSignature {
			if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setAllowedPref)", variations.SeedSignaturePref, seed.SeedSignature); err != nil {
				return errors.Wrapf(err, "failed to set %v", variations.SeedSignaturePref)
			}
			if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setAllowedPref)", variations.CompressedSeedPref, seed.CompressedSeed); err != nil {
				return errors.Wrapf(err, "failed to set %v", variations.CompressedSeedPref)
			}
			return errors.New("Local State has not updated with the test seed, injecting again")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		return errors.Wrap(err, "the test seed was not injected")
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
	var testSeed variations.SeedData
	if err := json.Unmarshal(b, &testSeed); err != nil {
		s.Fatal("Failed to unmarshal test seed: ", err)
	}

	// Log in, verify a production seed was fetched, and inject the test seed.
	// The injected test seed will take effect on the next start of Chrome.
	func() {
		// ChromeOS test images always have "unknown" browser channel since they are on testimage-channel.
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
			"--variations-server-url=https://clients4.google.com/chrome-variations/seed"))
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

	// Chrome is currently running with the test seed, but it will fetch a new seed for the next run.
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
	// Use a local http server to reduce dependencies on the network and external webpage contents.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	type tc struct {
		url           string
		text          string
		finder        *nodewith.Finder
		skiaGoldImage string
	}
	tests := []tc{
		{
			//data:text/html,<h1 id="success">Success</h1>
			url:    "data:text/html,%3Ch1%20id%3D%22success%22%3ESuccess%3C%2Fh1%3E",
			text:   "Success",
			finder: nodewith.Name("Success").Role(role.Heading),
		},
		{
			url:           filepath.Join(server.URL, "variations_test_index.html"),
			text:          "The Chromium Projects",
			finder:        nodewith.Name("The Chromium Projects").Role(role.Heading),
			skiaGoldImage: "finch_smoke_render_chromium_org_html",
		},
	}
	ui := uiauto.New(tconn)
	for _, t := range tests {
		func() {
			c, err := cr.NewConn(ctx, t.url)
			if err != nil {
				s.Fatalf("Failed to open Chrome browser with URL %v: %v", t.url, err)
			}
			defer c.Close()
			defer c.CloseTarget(ctx)

			if err := ui.WaitUntilExists(t.finder)(ctx); err != nil {
				s.Fatalf("Failed to find text %q on page %q: %v", t.text, t.url, err)
			}

			// Move cursor to the bottom-right of the screen before starting diff test to prevent any tool-tips from appearing on the browser.
			info, err := display.GetPrimaryInfo(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get info about the primary display: ", err)
			}
			pos := coords.Point{X: info.Bounds.Width, Y: info.Bounds.Height}
			s.Logf("X: %v, Y: %v", info.Bounds.Width, info.Bounds.Height)
			if err := mouse.Move(tconn, pos, 0)(ctx); err != nil {
				s.Fatal("Failed to move mouse to bottom right corner: ", err)
			}
			if len(t.skiaGoldImage) > 0 {
				d, err := screenshot.NewDifferFromChrome(ctx, s, cr, screenshot.Config{DefaultOptions: screenshot.Options{WindowWidthDP: 1024, WindowHeightDP: 720}})
				if err != nil {
					s.Fatal("Failed to start screen differ: ", err)
				}
				defer d.DieOnFailedDiffs()
				if err := d.Diff(ctx, "variations_smoke", nodewith.Role(role.RootWebArea), screenshot.Retries(5), screenshot.RetryInterval(time.Second))(ctx); err != nil {
					s.Error("Failed the skia gold diff: ", err)
				}
			}
		}()
	}
}
