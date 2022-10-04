// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package youtube

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	youtubePkg      = "com.google.android.youtube"
	playPauseBtnID  = youtubePkg + ":id/player_control_play_pause_replay_button"
	playBtnDesc     = "Play video"
	pauseBtnDesc    = "Pause video"
	playerViewID    = youtubePkg + ":id/player_view"
	bottomSheetID   = youtubePkg + ":id/design_bottom_sheet"
	optionsDialogID = youtubePkg + ":id/bottom_sheet_list_view"
	uiWaitTime      = 3 * time.Second // this is for arc-obj, not for uiauto.Context
	retryTimes      = 3
)

var appStartTime time.Duration

// YtApp defines the members related to youtube app.
type YtApp struct {
	tconn   *chrome.TestConn
	kb      *input.KeyboardEventWriter
	a       *arc.ARC
	d       *androidui.Device
	act     *arc.Activity
	outDir  string
	premium bool // Indicate if the account is premium.
}

// NewYtApp creates an instance of YtApp.
func NewYtApp(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, a *arc.ARC, d *androidui.Device, outDir string) *YtApp {
	return &YtApp{
		tconn:   tconn,
		kb:      kb,
		a:       a,
		d:       d,
		outDir:  outDir,
		premium: true,
	}
}

// OpenAndPlayVideo opens a video on youtube app.
func (y *YtApp) OpenAndPlayVideo(video VideoSrc) uiauto.Action {
	return func(ctx context.Context) (err error) {
		testing.ContextLog(ctx, "Open Youtube app")

		const (
			youtubeApp              = "Youtube App"
			youtubeAct              = "com.google.android.apps.youtube.app.WatchWhileActivity"
			closeDescription        = "Close"
			youtubeLogoDescription  = "YouTube Premium"
			accountImageDescription = "Account"
			noThanksText            = "NO THANKS"
			skipTrialText           = "SKIP TRIAL"
			accountImageID          = youtubePkg + ":id/image"
			searchButtonID          = youtubePkg + ":id/menu_item_1"
			searchEditTextID        = youtubePkg + ":id/search_edit_text"
			resultsViewID           = youtubePkg + ":id/results"
			dismissID               = youtubePkg + ":id/dismiss"
		)

		if appStartTime, y.act, err = cuj.OpenAppAndGetStartTime(ctx, y.tconn, y.a, youtubePkg, youtubeApp, youtubeAct); err != nil {
			return errors.Wrap(err, "failed to get app start time")
		}

		skipTrial := y.d.Object(androidui.ID(dismissID), androidui.Text(skipTrialText))
		if err := cuj.ClickIfExist(skipTrial, 5*time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'SKIP TRIAL' to skip premium trial")
		}

		closeButton := y.d.Object(androidui.Description(closeDescription))
		if err := cuj.ClickIfExist(closeButton, 5*time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Close' to the close premium trial prompt")
		}

		accountImage := y.d.Object(androidui.ID(accountImageID), androidui.DescriptionContains(accountImageDescription))
		if err := accountImage.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrap(err, "failed to check for Youtube app launched")
		}

		premiumLogo := y.d.Object(androidui.Description(youtubeLogoDescription))
		if err := premiumLogo.WaitForExists(ctx, uiWaitTime); err != nil {
			y.premium = false
			testing.ContextLog(ctx, "Current account is free account")
		}

		// Clear notification prompt if it exists.
		noThanksEle := y.d.Object(androidui.ID(dismissID), androidui.Text(noThanksText))
		if err := cuj.ClickIfExist(noThanksEle, 5*time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'NO THANKS' to clear notification prompt")
		}

		playVideo := func() error {
			testing.ContextLog(ctx, "Search and play video")

			searchButton := y.d.Object(androidui.ID(searchButtonID))
			if err := searchButton.Click(ctx); err != nil {
				return err
			}

			searchEditText := y.d.Object(androidui.ID(searchEditTextID))
			if err := cuj.FindAndClick(searchEditText, uiWaitTime)(ctx); err != nil {
				return errors.Wrap(err, "failed to find 'searchTextfield'")
			}

			if err := uiauto.Combine("type video url",
				y.kb.TypeAction(video.URL),
				y.kb.AccelAction("enter"),
			)(ctx); err != nil {
				return err
			}

			resultsView := y.d.Object(androidui.ID(resultsViewID))
			if err := resultsView.WaitForExists(ctx, uiWaitTime); err != nil {
				return errors.Wrap(err, "failed to find the results from video URL")
			}

			firstVideo := y.d.Object(androidui.DescriptionContains(video.Title))
			startTime := time.Now()
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				if err := cuj.FindAndClick(firstVideo, uiWaitTime)(ctx); err != nil {
					if strings.Contains(err.Error(), "click") {
						return testing.PollBreak(err)
					}
					return errors.Wrap(err, "failed to find 'First Video'")
				}

				testing.ContextLogf(ctx, "Elapsed time when waiting the video list: %.3f s", time.Since(startTime).Seconds())
				return nil
			}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 30 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to click first video")
			}
			return nil
		}

		if err := playVideo(); err != nil {
			return errors.Wrap(err, "failed to play video")
		}

		// It has been seen that low-end DUTs sometimes can take as much as 10-20 seconds to finish loading after clicking
		// on a video from the search results. Logic is added here to wait for the loading to complete before proceeding to
		// prevent unexpected errors.
		if err := y.waitForLoadingComplete(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for loading to complete")
		}

		if err := y.switchQuality(ctx, video.Quality); err != nil {
			return errors.Wrap(err, "failed to switch Quality")
		}

		return nil
	}
}

// switchQuality switches the video quality by continuous action.
// Due to the different response time of DUTs.
// We need to combine these actions in Poll to make switch quality works smoothly.
func (y *YtApp) switchQuality(ctx context.Context, resolution string) error {
	testing.ContextLogf(ctx, "Switch Quality to %q", resolution)

	const (
		qualityText       = "Quality"
		advancedText      = "Advanced"
		moreOptions       = youtubePkg + ":id/player_overflow_button"
		touchOutsideID    = youtubePkg + ":id/touch_outside"
		barRootID         = youtubePkg + ":id/action_bar_root"
		qualityListItemID = youtubePkg + ":id/list_item_text"
		titleResolutionID = youtubePkg + ":id/bottom_sheet_title_resolution"
	)

	if err := y.skipAds(ctx); err != nil {
		return errors.Wrap(err, "failed to skip YouTube ads")
	}

	startTime := time.Now()
	return testing.Poll(ctx, func(context.Context) error {
		optionsDialog := y.d.Object(androidui.ID(optionsDialogID))
		bottomSheet := y.d.Object(androidui.ID(bottomSheetID))
		err1 := optionsDialog.Exists(ctx)
		err2 := bottomSheet.Exists(ctx)
		qualityButtonIndex := 0
		// The playerView cannot be found/clicked when the options dialog (used for selecting quality) is present.
		// Press "Esc" to dismiss the options dialog, if present.
		if err1 == nil || err2 == nil {
			if err := y.kb.AccelAction("Esc")(ctx); err != nil {
				return errors.Wrap(err, "failed to press Esc to dismiss existing options dialog before clicking 'More options' button")
			}
			testing.ContextLog(ctx, "Dismissed existing options dialog before clicking 'More options' button")
			// There are two versions of the ARC UI for "More Options", which affects the index of the "Quality" button.
			if err2 == nil {
				qualityButtonIndex = 1
			}
		}

		playerView := y.d.Object(androidui.ID(playerViewID))
		if err := cuj.FindAndClick(playerView, uiWaitTime)(ctx); err != nil {
			return errors.Wrap(err, "failed to find/click the player view on switch quality")
		}

		moreBtn := y.d.Object(androidui.ID(moreOptions))
		if err := cuj.FindAndClick(moreBtn, uiWaitTime)(ctx); err != nil {
			return errors.Wrap(err, "failed to find/click the 'More options'")
		}

		// Capture screenshots before clicking the "Quality" option.
		if err := screenshot.Capture(ctx, filepath.Join(y.outDir, "before-click-quality.png")); err != nil {
			return errors.Wrap(err, "failed to capture screenshot before clicking 'Quality' option")
		}

		testing.ContextLogf(ctx, "Select %q option", qualityText)
		qualityButton := y.d.Object(androidui.ClassName("android.view.ViewGroup"), androidui.Clickable(true), androidui.Index(qualityButtonIndex))
		if err := cuj.FindAndClick(qualityButton, uiWaitTime)(ctx); err != nil {
			return err
		}

		// Capture screenshots after clicking the "Quality" option.
		if err := screenshot.Capture(ctx, filepath.Join(y.outDir, "after-click-quality.png")); err != nil {
			return errors.Wrap(err, "failed to capture screenshot after clicking 'Quality' option")
		}

		advancedButton := y.d.Object(androidui.Text(advancedText))
		if err := cuj.FindAndClick(advancedButton, uiWaitTime)(ctx); err != nil {
			return errors.Wrap(err, "failed to find/click the advanced option")
		}

		testing.ContextLogf(ctx, "Select target quality: %q", resolution)
		targetQualityButton := y.d.Object(androidui.ID(qualityListItemID), androidui.Text(resolution))
		// Immediately clicking the target button sometimes doesn't work.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep and wait before click resolution")
		}
		if err := cuj.FindAndClick(targetQualityButton, uiWaitTime)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the target quality")
		}

		testing.ContextLogf(ctx, "Elapsed time when switching quality: %.3f s", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute})
}

func (y *YtApp) waitForLoadingComplete(ctx context.Context) error {
	const (
		titleID               = youtubePkg + ":id/title"
		shareBtnText          = "Share"
		shareBtnTextID        = youtubePkg + ":id/button_text"
		sidebarID             = youtubePkg + ":id/video_metadata_layout"
		alternateElementClass = "android.view.ViewGroup"
		alternateTitleDesc    = "Expand description"
		alternateExpandDesc   = "Expand Mini Player"
	)
	videoTitle := y.d.Object(androidui.ID(titleID))
	shareBtn := y.d.Object(androidui.Text(shareBtnText), androidui.ID(shareBtnTextID))
	sidebar := y.d.Object(androidui.ID(sidebarID))
	// An alternate video title and share button are added here to support the two versions of UI trees observed across DUTs.
	// For details, please refer to b/206011393.
	alternateVideoTitle := y.d.Object(androidui.ClassName(alternateElementClass), androidui.Description(alternateTitleDesc))
	alternateExpandMenu := y.d.Object(androidui.ClassName(alternateElementClass), androidui.Description(alternateExpandDesc))
	alternateShareBtn := y.d.Object(androidui.ClassName(alternateElementClass), androidui.Description(shareBtnText))

	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := videoTitle.Exists(ctx); err != nil {
			testing.ContextLog(ctx, "Unable to find video title with expected UI tree: ", err)
			if alternateVideoTitle.Exists(ctx) != nil && alternateExpandMenu.Exists(ctx) != nil {
				return errors.New("still loading... video title not rendered")
			}
		}
		if err := shareBtn.Exists(ctx); err != nil {
			testing.ContextLog(ctx, "Unable to find share button with expected UI tree: ", err)
			if err2 := alternateShareBtn.Exists(ctx); err2 != nil {
				return errors.New("still loading... share button not rendered")
			}
		}
		if err := sidebar.Exists(ctx); err != nil {
			return errors.Wrap(err, "still loading... sidebar not rendered")
		}
		return nil
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 30 * time.Second})
}

func (y *YtApp) isPremiumAccount() bool {
	return y.premium
}

func (y *YtApp) checkYoutubeAppPIP(ctx context.Context) error {
	testing.ContextLog(ctx, "Check window state should be PIP")
	startTime := time.Now()

	ws, err := ash.GetARCAppWindowState(ctx, y.tconn, youtubePkg)
	if err != nil {
		return errors.Wrap(err, "can not get ARC App Window State")
	}
	if ws == ash.WindowStatePIP {
		testing.ContextLogf(ctx, "Elapsed time when checking PIP mode: %.3f s", time.Since(startTime).Seconds())
		return nil
	}

	waitForPipMode := func(ctx context.Context) error {
		return ash.WaitForARCAppWindowState(ctx, y.tconn, youtubePkg, ash.WindowStatePIP)
	}

	// Checking PIP mode sometimes doesn't work (e.g. if chrome window is not in fullscreen),
	// retry a few times to enable PIP mode.
	return uiauto.Retry(3,
		uiauto.Combine("change to pip mode",
			y.kb.AccelAction("Alt+="),
			waitForPipMode,
		),
	)(ctx)
}

// EnterFullscreen switches youtube video to fullscreen.
func (y *YtApp) EnterFullscreen(ctx context.Context) error {
	// If the youtube app is already in fullscreen, skip the process to go fullscreen.
	if err := ash.WaitForCondition(ctx, y.tconn, func(w *ash.Window) bool {
		return w.Title == YoutubeWindowTitle && w.State == ash.WindowStateFullscreen
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err == nil {
		return nil
	}

	waitWindowStateFullscreen := func(ctx context.Context) error {
		return waitWindowStateFullscreen(ctx, y.tconn, YoutubeWindowTitle)
	}

	const fullscreenDesc = "Enter fullscreen"
	fsBtn := y.d.Object(androidui.Description(fullscreenDesc))
	playerView := y.d.Object(androidui.ID(playerViewID))

	startTime := time.Now()
	if err := uiauto.NamedAction("make Youtube app fullscreen",
		uiauto.Retry(retryTimes, uiauto.Combine("enter fullscreen",
			cuj.FindAndClick(playerView, uiWaitTime),
			cuj.FindAndClick(fsBtn, uiWaitTime),
			waitWindowStateFullscreen,
		)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to enter fullscreen")
	}

	testing.ContextLogf(ctx, "Elapsed time when doing enter fullscreen %.3f s", time.Since(startTime).Seconds())
	return nil
}

// PauseAndPlayVideo verifies video playback on youtube app.
func (y *YtApp) PauseAndPlayVideo(ctx context.Context) error {
	testing.ContextLog(ctx, "Pause and play video")
	const sleepTime = 3 * time.Second

	playerView := y.d.Object(androidui.ID(playerViewID))
	pauseBtn := y.d.Object(androidui.ID(playPauseBtnID), androidui.Description(pauseBtnDesc))
	playBtn := y.d.Object(androidui.ID(playPauseBtnID), androidui.Description(playBtnDesc))

	// The video should be playing at this point. However, we'll double check to make sure
	// as we have seen a few cases where the video became paused automatically.
	if err := y.ensureVideoPlaying(ctx, playerView, playBtn); err != nil {
		return errors.Wrap(err, "failed to ensure video is playing before pausing")
	}

	startTime := time.Now()
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := cuj.FindAndClick(playerView, uiWaitTime)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find/click the player view in %s", uiWaitTime)
		}

		if err := cuj.FindAndClick(pauseBtn, uiWaitTime)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find/click the pause button in %s", uiWaitTime)
		}

		if err := playBtn.WaitForExists(ctx, 2*time.Second); err != nil {
			return errors.Wrap(err, "failed to find the play button in 2s")
		}

		// Immediately clicking the target button sometimes doesn't work.
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			return errors.Wrap(err, "failed to sleep before clicking play button")
		}
		if err := playBtn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the play button")
		}
		if err := pauseBtn.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrapf(err, "failed to find the pause button in %s", uiWaitTime)
		}

		// Keep the video playing for a short time.
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			return errors.Wrap(err, "failed to sleep while video is playing")
		}

		testing.ContextLogf(ctx, "Elapsed time when checking the playback status of youtube app: %.3f s", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute})
}

func (y *YtApp) ensureVideoPlaying(ctx context.Context, playerView, playBtn *androidui.Object) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := cuj.FindAndClick(playerView, 2*time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to find/click the player view in 2s")
		}
		if err := playBtn.WaitForExists(ctx, 2*time.Second); err == nil {
			testing.ContextLog(ctx, "Video is paused; resuming video")
			return playBtn.Click(ctx)
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second})
}

// IsPlaying checks if the video is playing now.
func (y *YtApp) IsPlaying() uiauto.Action {
	return func(ctx context.Context) error {
		playerView := y.d.Object(androidui.ID(playerViewID))
		playBtn := y.d.Object(androidui.ID(playPauseBtnID), androidui.Description(playBtnDesc))
		testing.ContextLog(ctx, "Verify the video is playing")
		return y.ensureVideoPlaying(ctx, playerView, playBtn)
	}
}

// dumpAppInfo dumps Youtube app version name and code.
func dumpAppInfo(ctx context.Context, a *arc.ARC, d *ui.Device, appPkgName string) (string, error) {
	var versionName, versionCode string
	out, err := a.Command(ctx, "dumpsys", "package", appPkgName).Output()
	if err == nil {
		versionNamePrefix, versionCodePrefix := "versionName=", "versionCode="
		output := string(out)
		splitOutput := strings.Split(output, "\n")
		for splitLine := range splitOutput {
			if strings.Contains(splitOutput[splitLine], versionNamePrefix) {
				versionName = strings.Split(splitOutput[splitLine], "=")[1]
			}
			if strings.Contains(splitOutput[splitLine], versionCodePrefix) {
				versionCodeLine := strings.Fields(splitOutput[splitLine])[0]
				versionCode = strings.Split(versionCodeLine, "=")[1]
			}
			if versionCode != "" && versionName != "" {
				break
			}
		}
		if versionName == "" {
			err = errors.Errorf("%s is not found in the output", versionNamePrefix)
		}
		if versionCode == "" {
			err = errors.Errorf("%s is not found in the output", versionCodePrefix)
		}
	}
	testing.ContextLogf(ctx, "Youtube app version: %s; Version code: %s", versionName, versionCode)
	return versionName, err
}

func (y *YtApp) skipAds(ctx context.Context) error {
	if y.premium {
		testing.ContextLog(ctx, "Currently using Premium account; no need to check for ads")
		return nil
	}

	const (
		visitAdvertiserText = "Visit advertiser"
		skipAdsID           = youtubePkg + ":id/skip_ad_button"
	)

	visitAdvertiserBtn := y.d.Object(androidui.Text(visitAdvertiserText))
	skipAdsBtn := y.d.Object(androidui.ID(skipAdsID))

	testing.ContextLog(ctx, "Checking for YouTube ads")
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := visitAdvertiserBtn.WaitForExists(ctx, uiWaitTime); err != nil && androidui.IsTimeout(err) {
			return nil
		}
		if err := skipAdsBtn.Exists(ctx); err != nil {
			return errors.Wrap(err, "'Skip ads' button not available yet")
		}
		if err := skipAdsBtn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Skip ads'")
		}
		return errors.New("have not determined whether the ad has been skipped successfully")
	}, &testing.PollOptions{Timeout: time.Minute})
}

// Close closes the resources related to video.
func (y *YtApp) Close(ctx context.Context) {
	if y.act != nil {
		y.act.Stop(ctx, y.tconn)
		y.act.Close()
		y.act = nil
	}
}
