// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VP9VideoDecoder,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies whether VP9 decoder is supported or not",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.X86()),
		Fixture:      "chromeLoggedIn",
	})
}

func VP9VideoDecoder(ctx context.Context, s *testing.State) {
	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	var videoSource = videocuj.VideoSrc{
		URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
		Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		Quality: "2160p60",
	}

	uiHandler, err := cuj.NewClamshellActionHandler(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard input: ", err)
	}
	defer kb.Close()

	cui := uiauto.New(tconn)
	isExtendedDisplay := false
	videoApp := videocuj.NewYtWeb(cr.Browser(), tconn, kb, videoSource, isExtendedDisplay, cui, uiHandler)
	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
	}
	defer videoApp.Close(cleanupCtx)

	fullScreenButton := nodewith.Name("Full screen (f)").Role(role.Button)
	statsForNerdsItem := nodewith.Name("Stats for nerds").Role(role.MenuItem)
	codecsVp9Text := nodewith.NameStartingWith(" vp09").Role(role.StaticText)
	selectedResolutionText := nodewith.NameStartingWith(" 3840x2160@60").Role(role.StaticText)
	if err := uiauto.Combine("check and validate for video codecs",
		cui.RightClick(fullScreenButton),
		cui.WaitUntilExists(statsForNerdsItem),
		cui.LeftClick(statsForNerdsItem),
		cui.WaitUntilExists(codecsVp9Text),
		cui.WaitUntilExists(selectedResolutionText),
	)(ctx); err != nil {
		s.Fatal("Failed to check for video codecs: ", err)
	}
}
