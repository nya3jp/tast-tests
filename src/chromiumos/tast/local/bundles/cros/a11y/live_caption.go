// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package a11y

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
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
		Func: LiveCaption,
		Desc: "Checks live caption works",
		Contacts: []string{
			"alanlxl@chromium.org",
			"amoylan@chromium.org",
		},
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome", "ondevice_speech"},
		Attr:         []string{"group:mainline", "informational"},
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

	// Launch chrome.
	cr, err := chrome.New(
		ctx,
		chrome.ExtraArgs("--autoplay-policy=no-user-gesture-required"), // Allow media autoplay.
		chrome.EnableFeatures("OnDeviceSpeechRecognition"),
	)
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
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
		s.Fatal("Failed to turn on live caption toggle")
	}

	// Open the test page and play the audio.
	conn, err := cr.NewConn(ctx, server.URL+"/live_caption.html")
	if err != nil {
		s.Fatal("Failed to open test web page: ", err)
	}
	defer conn.Close()

	liveCaptionBubble := nodewith.ClassName("CaptionBubbleFrameView")
	liveCaptionContent := nodewith.Name("Hello").Role(role.StaticText)

	// Not use uiauto.Combine because we want to distinguish between "timeout" and "content is wrong".
	// Because liveCaptionBubble exists only when live caption emits content, if the expected
	// liveCaptionContent doesn't show, we know there's wrong content.
	if err := ui.WaitUntilExists(liveCaptionBubble)(ctx); err != nil {
		s.Fatal("Failed to wait for live caption bubble: ", err)
	}

	if err := ui.WaitUntilExists(liveCaptionContent)(ctx); err != nil {
		s.Fatal("Failed to wait for correct live caption content: ", err)
	}

	if err := ui.WaitUntilGone(liveCaptionBubble)(ctx); err != nil {
		s.Fatal("Failed to wait for live caption content disappear: ", err)
	}
}
