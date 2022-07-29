// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package youtube

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/media/webmedia"
	"chromiumos/tast/testing"
)

// YouTube holds resources to control YouTube web page.
type YouTube struct {
	*webmedia.Video
	ui           *uiauto.Context
	playerFinder *nodewith.Finder
}

// New returns a new YouTube instance.
func New(tconn *chrome.TestConn, url string) *YouTube {
	windowRoot := nodewith.Ancestor(nodewith.Role(role.Window).NameContaining("YouTube").HasClass("BrowserFrame"))
	playerFinder := windowRoot.Name("YouTube Video Player").Role(role.GenericContainer)
	return &YouTube{
		Video: webmedia.New(
			tconn,
			url,
			"document.querySelector('video')",
			playerFinder,
		),
		ui:           uiauto.New(tconn),
		playerFinder: playerFinder,
	}
}

// Play plays the youtube.
func (yt *YouTube) Play(ctx context.Context) error {
	return uiauto.Combine("play YouTube video",
		yt.Video.Play,
		yt.ClearPrompt,
		yt.SkipAd,
	)(ctx)
}

// ClearPrompt clears prompts.
func (yt *YouTube) ClearPrompt(ctx context.Context) error {
	if err := yt.WaitUntilVideoReady(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for video being loaded")
	}

	// If prompted to open in YouTube app, instruct device to stay in Chrome.
	stayInChrome := nodewith.Name("Stay in Chrome").Role(role.Button)
	if err := uiauto.IfSuccessThen(
		yt.ui.WithTimeout(5*time.Second).WaitUntilExists(stayInChrome),
		func(ctx context.Context) error {
			testing.ContextLog(ctx, "Dialog popped up and asked whether to switch to YouTube app")

			rememberMyChoice := nodewith.Name("Remember my choice").Role(role.CheckBox)
			if err := uiauto.Combine("clear prompts",
				yt.ui.LeftClick(rememberMyChoice),
				yt.ui.LeftClick(stayInChrome),
			)(ctx); err != nil {
				return err
			}

			testing.ContextLog(ctx, "Instructed device to stay on YouTube web")
			return nil
		},
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to instruct device to stay on YouTube web")
	}
	return nil

}

// SkipAd checks and skips ads.
func (yt *YouTube) SkipAd(ctx context.Context) error {
	// Ensure video is playing before checking ads.
	if err := yt.Video.Play(ctx); err != nil {
		return errors.Wrap(err, "failed to start playing video")
	}

	skipAdExpr := `() => new Promise((resolve, reject) => {
		adLinkExists = !!document.querySelector(".ytp-ad-button.ytp-ad-visit-advertiser-button.ytp-ad-button-link");
		skipAdBtn = document.querySelector(".ytp-ad-skip-button.ytp-button");
		skipAdExists = !!skipAdBtn;

		if (!adLinkExists && !skipAdExists) {
			resolve();
		} else {
			reject(skipAdBtn.click());
		}
	})`

	// According to YouTube, non-skippable video ads could be up to 20 seconds.
	// (https://support.google.com/youtube/answer/2467968)
	const adTimeout = 30 * time.Second
	conn := yt.GetConn()

	return testing.Poll(ctx, func(ctx context.Context) error {
		return conn.Call(ctx, nil, skipAdExpr)
	}, &testing.PollOptions{Timeout: adTimeout})
}
