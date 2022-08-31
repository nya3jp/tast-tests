// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/apputil/youtube"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         YoutubeAppPlayingWhenUnfocused,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "ARC++ Youtube video should not be paused while window focus shifted",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"edgar.chang@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "arcBootedWithPlayStore",
		Timeout:      3*time.Minute + apputil.InstallationTimeout,
	})
}

// YoutubeAppPlayingWhenUnfocused keep youtube app playing while window focus shifted
func YoutubeAppPlayingWhenUnfocused(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard controller: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	yt, err := youtube.NewApp(ctx, kb, tconn, a)
	if err != nil {
		s.Fatal("Failed to create arc resource: ", err)
	}
	defer yt.Close(cleanupCtx, cr, s.HasError, s.OutDir())

	if err := yt.Install(ctx); err != nil {
		s.Fatal("Failed to install youtube app: ", err)
	}

	if _, err = yt.Launch(ctx); err != nil {
		s.Fatal("Failed to launch youtube app: ", err)
	}

	if err := yt.Play(ctx, apputil.NewMedia(
		"https://www.youtube.com/watch?v=JE3-LkMqBfM",
		"Whale Songs and AI, for everyone to explore",
	)); err != nil {
		s.Fatal("Failed to play media element: ", err)
	}

	if err := quicksettings.OpenSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to try to shift focus by opening uber tray and setting: ", err)
	}
	defer apps.Close(cleanupCtx, tconn, apps.Settings.ID)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "setting_ui_dump")

	if w, err := ash.GetARCAppWindowInfo(ctx, yt.Tconn, yt.PkgName); err != nil {
		s.Fatal("Failed to get ARC UI window info: ", err)
	} else if w.HasFocus {
		s.Fatal("Failed to focus on another window")
	}

	if isPlaying, err := yt.IsPlaying(ctx); err != nil {
		s.Fatal("Failed to check youtube app playing status: ", err)
	} else if !isPlaying {
		s.Fatal("Failed to complete test, youtube is not playing")
	}
}
