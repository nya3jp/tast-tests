// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package youtube contains local Tast tests that exercise youtube.
package youtube

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// AppName is the name of ARC app.
	AppName = "YouTube"
	// PkgName is the package name of ARC app.
	PkgName = "com.google.android.youtube"

	idPrefix = PkgName + ":id/"

	shortTimeout     = 2 * time.Second
	defaultUITimeout = 15 * time.Second
)

// Youtube holds resources of ARC app Youtube.
type Youtube struct {
	*apputil.App

	// isPremium denotes whether a premium account is being used.
	isPremium bool
}

// NewApp returns Youtube instance.
func NewApp(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC) (*Youtube, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, AppName, PkgName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create arc resource")
	}

	return &Youtube{App: app}, nil
}

// ClearPrompts clears the start up prompts of Youtube app.
func (yt *Youtube) ClearPrompts(ctx context.Context) error {
	testing.ContextLog(ctx, "Clearing prompts")

	dismissID := idPrefix + "dismiss"

	skipTrial := yt.Device.Object(ui.ID(dismissID), ui.Text("SKIP TRIAL"))
	if err := apputil.ClickIfExist(skipTrial, shortTimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to click 'SKIP TRIAL' to skip premium trial")
	}

	closePrompt := yt.Device.Object(ui.Description("Close"), ui.ClassName("android.view.ViewGroup"))
	if err := apputil.ClickIfExist(closePrompt, shortTimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to click 'Close' to close premium extend trial")
	}

	noThanksEle := yt.Device.Object(ui.ID(dismissID), ui.Text("NO THANKS"))
	if err := apputil.ClickIfExist(noThanksEle, shortTimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to click 'NO THANKS' to clear notification prompt")
	}

	premiumLogo := yt.Device.Object(ui.Description("YouTube Premium"))
	if isExist, err := apputil.CheckObjectExists(ctx, premiumLogo, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to check premium logo")
	} else if isExist {
		yt.isPremium = true
		testing.ContextLog(ctx, "Current account is a premium account")
	}

	return nil
}

// Play opens and plays a video on youtube app.
func (yt *Youtube) Play(ctx context.Context, media *apputil.Media) (err error) {
	if err := yt.ClearPrompts(ctx); err != nil {
		return errors.Wrap(err, "failed to clear prompts")
	}

	testing.ContextLog(ctx, "Search and play video")

	searchButton := yt.Device.Object(ui.ID(idPrefix + "menu_item_1"))
	searchEditText := yt.Device.Object(ui.ID(idPrefix + "search_edit_text"))
	resultsView := yt.Device.Object(ui.ID(idPrefix + "results"))

	if err := uiauto.Combine("search video by url",
		apputil.FindAndClick(searchButton, defaultUITimeout),
		apputil.FindAndClick(searchEditText, defaultUITimeout),
		yt.KB.TypeAction(media.Query),
		yt.KB.AccelAction("enter"),
		apputil.WaitForExists(resultsView, defaultUITimeout),
	)(ctx); err != nil {
		return err
	}

	resultVideo := yt.Device.Object(ui.DescriptionContains(media.Subtitle))
	startTime := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := apputil.FindAndClick(resultVideo, defaultUITimeout)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find %q", media.Subtitle)
		}

		testing.ContextLog(ctx, "Elapsed time when waiting the video list: ", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed to click %q", media.Subtitle)
	}

	// Skip AD if exist.
	if !yt.isPremium {
		skipAds := yt.Device.Object(ui.ID(idPrefix + "skip_ad_button"))
		if err := apputil.ClickIfExist(skipAds, defaultUITimeout)(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Skip Ads'")
		}
	}

	return nil
}
