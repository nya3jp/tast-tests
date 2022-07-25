// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package a11y

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const liveCaptionSubPageURL = "manageAccessibility/captions"
const liveCaptionToggleName = "Live Caption"

func init() {
	testing.AddTest(&testing.Test{
		Func:         LiveCaption,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks live caption works",
		Contacts: []string{
			"alanlxl@chromium.org",
			"amoylan@chromium.org",
			"chrome-knowledge-eng@google.com",
		},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "ondevice_speech"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Val:               browser.TypeLacros,
		}},
		Data: []string{
			"live_caption.html",
			"voice_en_hello.wav",
		},
	})
}

func LiveCaption(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup test HTTP server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Launch browser.
	bt := s.Param().(browser.Type)
	cr, err := browserfixt.NewChrome(ctx, bt, lacrosfixt.NewConfig(),
		chrome.ExtraArgs("--autoplay-policy=no-user-gesture-required"), // Allow media autoplay.
		chrome.EnableFeatures("OnDeviceSpeechRecognition"))
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)

	// Turn on Live Caption toggle via OS settings.
	captionsHeading := nodewith.NameStartingWith("Captions").Role(role.Heading).Ancestor(ossettings.WindowFinder)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, liveCaptionSubPageURL, ui.Exists(captionsHeading))
	if err != nil {
		s.Fatal("Failed to open setting page: ", err)
	}
	if err := uiauto.Combine("turn on live caption",
		ui.WaitUntilExists(nodewith.Name(liveCaptionToggleName).Role(role.ToggleButton)),
		settings.SetToggleOption(cr, liveCaptionToggleName, true),
	)(ctx); err != nil {
		s.Fatal("Failed to turn on live caption toggle: ", err)
	}

	// Wait until dlc libsoda and libsoda-model-en-us are installed.
	if err := testing.Poll(ctx, a11y.VerifySodaInstalled, &testing.PollOptions{Timeout: 2 * time.Minute, Interval: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for libsoda dlc to be installed: ", err)
	}

	// Open the test page and play the audio.
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, bt, server.URL+"/live_caption.html")
	if err != nil {
		s.Fatal("Failed to open test web page: ", err)
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()

	audioPlayButton := nodewith.Name("play").Role(role.Button)
	audioPauseButton := nodewith.Name("pause").Role(role.Button)
	liveCaptionBubble := nodewith.ClassName("CaptionBubbleFrameView")
	liveCaptionContent := nodewith.Name("Hello").Role(role.StaticText)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := uiauto.Combine("Play the audio",
			ui.WaitUntilExists(audioPlayButton),
			ui.LeftClick(audioPlayButton),
			ui.WaitUntilExists(audioPauseButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to play the audio")
		}

		// Not use uiauto.Combine because we want to distinguish between "timeout" and "content is wrong".
		// Because liveCaptionBubble exists only when live caption emits content, if the expected
		// liveCaptionContent doesn't show, we know there's wrong content.
		// TODO: Confirm live caption is enabled for lacros. otherwise, lacros variant test will fail here.
		if err := ui.WaitUntilExists(liveCaptionBubble)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for live caption bubble to show")
		}

		if err := ui.WaitUntilExists(liveCaptionContent)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for correct content")
		}

		if err := ui.WaitUntilGone(liveCaptionBubble)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for live caption bubble disappear")
		}

		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second, Interval: 10 * time.Second}); err != nil {
		s.Fatal("Failed to verify live caption bubble: ", err)
	}
}
