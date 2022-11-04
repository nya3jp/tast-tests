// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/apputil/youtube"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcvmAppPower,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Install and interact with ARC++ app, then measure power consumption",
		Contacts: []string{
			"jingmuli@google.com",
		},
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      2*time.Minute + apputil.InstallationTimeout,
		Fixture:      "arcBootedWithPlayStore",
	})
}

const (
	ytAppLink    = "https://www.youtube.com/watch?v=JE3-LkMqBfM"
	ytAppVideo   = "Whale Songs and AI, for everyone to explore"
	sleepingTime = 2 * time.Minute
)

func ArcvmAppPower(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}

	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var appName = youtube.AppName
	var media = apputil.NewMedia(ytAppLink, ytAppVideo)

	var app apputil.ARCMediaPlayer
	var appPkgName = youtube.PkgName

	app, _err := youtube.NewApp(ctx, kb, tconn, a)

	if _err != nil {
		s.Fatal("Failed to create media app instance: ", err)
	}

	if err := app.Install(ctx); err != nil {
		s.Fatal("Failed to install: ", err)
	}

	if _, err := app.Launch(ctx); err != nil {
		s.Fatal("Failed to launch: ", err)
	}

	// Set window state to be fullscreen
	if _, err := ash.SetARCAppWindowStateAndWait(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
		s.Fatalf("Failed to set %s window state to normal: %v", appPkgName, err)
	}

	if err := app.Play(ctx, media); err != nil {
		s.Fatal("Failed to play media: ", err)
	}

	testing.Sleep(ctx, sleepingTime)
	defer cancel()
	defer app.Close(cleanupCtx, cr, s.HasError, filepath.Join(s.OutDir(), appName))
}
