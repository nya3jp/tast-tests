// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnableA11yImageLabels,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verifies that the AccessibilityImageLabels policy works as intended",
		Contacts: []string{
			"eariassoto@google.com", // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data: []string{
			"enable_a11y_image_labels_index.html",
			"enable_a11y_image_labels_image.jpg",
		},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AccessibilityImageLabelsEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func EnableA11yImageLabels(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(cleanupCtx)

	// Start a server that will serve a test webpage.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		name             string
		value            *policy.AccessibilityImageLabelsEnabled
		expectationRegex string
	}{
		{
			name:             "true",
			value:            &policy.AccessibilityImageLabelsEnabled{Val: true},
			expectationRegex: "(Appears to be|Getting description).*",
		},
		{
			name:             "false",
			value:            &policy.AccessibilityImageLabelsEnabled{Val: false},
			expectationRegex: ".*missing image.*",
		},
		{
			name:             "unset",
			value:            &policy.AccessibilityImageLabelsEnabled{Stat: policy.StatusUnset},
			expectationRegex: ".*missing image.*",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup before each test.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies with verification.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open the test web page in a browser.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, server.URL+"/enable_a11y_image_labels_index.html")
			if err != nil {
				s.Fatal("Failed to open test webpage: ", err)
			}
			defer conn.Close()

			// Set up ChromeVox.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			sm, cleanup, err := setUpChromeVox(ctx, tconn, cr)
			defer func() {
				if err := cleanup(cleanupCtx); err != nil {
					s.Fatal("Failed to clean up ChromeVox: ", err)
				}
			}()
			if err != nil {
				s.Fatal("Failed to set up ChromeVox: ", err)
			}

			// Dump the UI tree to help debug if something goes wrong.
			// We do this after laying out the entire UI for the test, to generate the
			// dump before any of the UI elements are closed.
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Make sure we display the webpage and the image on screen before
			// attempting to select it.
			ui := uiauto.New(tconn)
			testImg := nodewith.Role(role.Image).Ancestor(nodewith.Role(role.RootWebArea).Name("Image labels test page"))
			if err := ui.WaitUntilExists(testImg)(ctx); err != nil {
				s.Fatal("Failed to show image on screen: ", err)
			}

			// Test that we get the expected spoken feedback.
			if err := a11y.PressKeysAndConsumeExpectations(ctx, sm,
				[]string{"Search+Right"},
				[]a11y.SpeechExpectation{a11y.NewRegexExpectation(param.expectationRegex)},
			); err != nil {
				s.Fatal("Got unexpected description message: ", err)
			}
		})
	}

}

func setUpChromeVox(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*a11y.SpeechMonitor, func(context.Context) error, error) {
	// Set up a data structure to hold all cleanup actions that we will merge
	// into a cleanup function returned to the caller.
	var cleanups []func(context.Context) error

	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, true); err != nil {
		return nil, mergeCleanups(cleanups), errors.Errorf("failed to enable ChromeVox: %s", err)
	}
	cleanups = append(cleanups, func(cleanupCtx context.Context) error {
		if err := a11y.SetFeatureEnabled(cleanupCtx, tconn, a11y.SpokenFeedback, false); err != nil {
			return errors.Errorf("failed to disable ChromeVox: %s", err)
		}
		return nil
	})

	cvConn, err := a11y.NewChromeVoxConn(ctx, cr)
	if err != nil {
		return nil, mergeCleanups(cleanups), errors.Errorf("failed to connect to the ChromeVox background page: %s", err)
	}
	cleanups = append(cleanups, func(cleanupCtx context.Context) error {
		cvConn.Close()
		return nil
	})

	if err := cvConn.SetVoice(ctx, a11y.VoiceData{ExtID: a11y.GoogleTTSExtensionID, Locale: "en-US"}); err != nil {
		return nil, mergeCleanups(cleanups), errors.Errorf("failed to set the ChromeVox voice: %s", err)
	}

	if err := a11y.SetTTSRate(ctx, tconn, 5.0); err != nil {
		return nil, mergeCleanups(cleanups), errors.Errorf("failed to change TTS rate: %s", err)
	}
	cleanups = append(cleanups, func(cleanupCtx context.Context) error {
		a11y.SetTTSRate(cleanupCtx, tconn, 1.0)
		return nil
	})

	ed := a11y.TTSEngineData{
		ExtID:                     a11y.GoogleTTSExtensionID,
		UseOnSpeakWithAudioStream: false,
	}
	sm, err := a11y.RelevantSpeechMonitor(ctx, cr, tconn, ed)
	if err != nil {
		return nil, mergeCleanups(cleanups), errors.Errorf("failed to connect to the TTS background page %s", err)
	}
	cleanups = append(cleanups, func(cleanupCtx context.Context) error {
		sm.Close()
		return nil
	})

	rootWebArea := nodewith.Role(role.RootWebArea).First()
	if err = cvConn.WaitForFocusedNode(ctx, tconn, rootWebArea); err != nil {
		return nil, mergeCleanups(cleanups), errors.Errorf("failed to wait for initial ChromeVox focus: %s", err)
	}

	return sm, mergeCleanups(cleanups), nil
}

func mergeCleanups(cleanups []func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		for i := len(cleanups) - 1; i >= 0; i-- {
			if err := cleanups[i](ctx); err != nil {
				return err
			}
		}
		return nil
	}
}
