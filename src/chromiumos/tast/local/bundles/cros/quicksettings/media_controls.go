// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/browser/media/youtubemusic"
	"chromiumos/tast/local/chrome/uiauto/browser/media/youtubeweb"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

type mediaControlsTestParam struct {
	tabletMode  bool
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaControls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify Media controls UI and functionality in quick settings",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"cros-status-area-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "clamshell_mode",
				Fixture: "chromeLoggedIn",
				Val: mediaControlsTestParam{
					tabletMode:  false,
					browserType: browser.TypeAsh,
				},
			}, {
				Name:    "tablet_mode",
				Fixture: "chromeLoggedIn",
				Val: mediaControlsTestParam{
					tabletMode:  true,
					browserType: browser.TypeAsh,
				},
			}, {
				Name:              "clamshell_mode_lacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           "lacros",
				Val: mediaControlsTestParam{
					tabletMode:  false,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "tablet_mode_lacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           "lacros",
				Val: mediaControlsTestParam{
					tabletMode:  true,
					browserType: browser.TypeLacros,
				},
			},
		},
		Timeout: 3 * time.Minute,
	})
}

type testResource struct {
	tconn *chrome.TestConn
	ui    *uiauto.Context
}

type webMedia struct {
	player
	title string
	url   string
}

// player defines the interface to operate web media player.
type player interface {
	// Open opens a web page with media player in it.
	Open(ctx context.Context, br *browser.Browser, url string) error
	// Close closes the web page.
	Close(ctx context.Context, hasError func() bool) error
	// Play plays the media.
	Play(ctx context.Context) error
	// WaitUntilPlaying waits the media status until it is playing.
	WaitUntilPlaying(ctx context.Context) error
	// WaitUntilPaused waits the media status until it is paused.
	WaitUntilPaused(ctx context.Context) error
	// RetrieveCurrentTime retrieves the current time of the media player.
	RetrieveCurrentTime(ctx context.Context) (time.Duration, error)
}

func newYoutubeMusicWebMedia(tconn *chrome.TestConn, browserType browser.Type, cr *chrome.Chrome, outDir string) *webMedia {
	return &webMedia{
		player: youtubemusic.New(tconn, browserType, cr, outDir),
		title:  "Blank Space",
		url:    "https://music.youtube.com/watch?v=e-ORhEE9VVg&list=RDAMVMe-ORhEE9VVg",
	}
}

func newYoutubeWebMedia(tconn *chrome.TestConn, browserType browser.Type, cr *chrome.Chrome, outDir string) *webMedia {
	return &webMedia{
		player: youtubeweb.New(tconn, browserType, cr, outDir),
		title:  `"Go Beyond" 4K Ultra HD Time Lapse`,
		url:    "https://www.youtube.com/watch?v=suWsd372pQE",
	}
}

// MediaControls verifies media controls UI and functionality in quick settings.
func MediaControls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	resource := &testResource{
		tconn: tconn,
		ui:    uiauto.New(tconn),
	}

	s.Log("Verify user do not have any media playing in the beginning")
	if err := verifyMediaControlsDoesNotExist(ctx, resource); err != nil {
		s.Fatal("Failed to verify media controls not in the shelf or quick settings: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	param := s.Param().(mediaControlsTestParam)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, param.tabletMode)
	if err != nil {
		s.Fatalf("Failed to ensure the tablet mode is set to %v: %v", param.tabletMode, err)
	}
	defer cleanup(cleanupCtx)

	browserType := param.browserType
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browserType)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// TODO(b/236799853): Remove this block after lacros node loaction mismatching issue resolved.
	if browserType == browser.TypeLacros {
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.WindowType == ash.WindowTypeLacros })
		if err != nil {
			s.Fatal("Failed to get browser window: ", err)
		}
		if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized); err != nil {
			s.Fatal("Failed to set lacros window to be maximized: ", err)
		}
	}

	for hasPlaylist, media := range map[bool]*webMedia{
		true:  newYoutubeMusicWebMedia(tconn, browserType, cr, s.OutDir()),
		false: newYoutubeWebMedia(tconn, browserType, cr, s.OutDir()),
	} {
		func() {
			if err := media.player.Open(ctx, br, media.url); err != nil {
				s.Fatal("Failed to open web page: ", err)
			}
			defer media.player.Close(cleanupCtx, s.HasError)

			if err := media.player.Play(ctx); err != nil {
				s.Fatal("Failed to play media: ", err)
			}

			// The media controls icon might appear in the shelf by default. Unpin it to move it to the quick settings.
			if err := uiauto.IfSuccessThen(
				resource.ui.WaitUntilExists(quicksettings.PinnedMediaControls),
				quicksettings.UnpinMediaControlsPod(tconn),
			)(ctx); err != nil {
				s.Fatal("Failed to move the media controls to the quick settings: ", err)
			}

			if err := quicksettings.Show(ctx, tconn); err != nil {
				s.Fatal("Failed to show the quick settings: ", err)
			}
			defer quicksettings.Hide(cleanupCtx, tconn)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "media_controls")

			// The media controls of a media with playlist is different with a media without playlist.
			// This test only verifies the extra media controls on a media with playlist.
			// The extra controls include image view, previous/next track button on the main page of media controls,
			// and play, pause, forward, backward buttons on the subpage of media controls.
			verifyExtraControls := hasPlaylist
			if err := verifyMediaControls(ctx, resource, media.player, media.title, verifyExtraControls); err != nil {
				s.Fatal("Failed to verify media controls UI and functions: ", err)
			}
		}()
	}

	s.Log("Verify the media resoures are closed")
	if err := verifyMediaControlsDoesNotExist(ctx, resource); err != nil {
		s.Fatal("Failed to verify media controls not in the shelf or quick settings: ", err)
	}
}

// verifyMediaControlsDoesNotExist verifies media controls icon is not in the shelf and the quick settings panel.
func verifyMediaControlsDoesNotExist(ctx context.Context, res *testResource) error {
	if err := res.ui.WaitUntilGone(quicksettings.PinnedMediaControls)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for media controls to disappear in the shelf")
	}
	testing.ContextLog(ctx, "Media controls icon is not in the shelf")

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := quicksettings.Show(ctx, res.tconn); err != nil {
		return errors.Wrap(err, "failed to show the quick settings")
	}
	defer quicksettings.Hide(cleanupCtx, res.tconn)

	if err := res.ui.WaitUntilGone(quicksettings.MediaControlsPod)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for media controls to disappear in the quick settings")
	}
	testing.ContextLog(ctx, "Media controls icon is not in the quick settings")

	return nil
}

// verifyMediaControls verifies media controls UI and functionality in quick settings.
// verifyExtraControls is the parameter to decide if the extra controls need to be verified on the media controls.
func verifyMediaControls(ctx context.Context, res *testResource, player player, title string, verifyExtraControls bool) error {
	for _, node := range []*nodewith.Finder{
		quicksettings.MediaControlsLabel.Name(title),
		quicksettings.MediaControlsPauseBtn,
	} {
		if err := res.ui.WaitUntilExists(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to find UI node on the media controls")
		}
	}

	if !verifyExtraControls {
		return nil
	}

	if err := verifyImageOnMediaControls(ctx, res.ui); err != nil {
		return errors.Wrap(err, "failed to find image view on the media controls")
	}

	for _, node := range []*nodewith.Finder{
		quicksettings.MediaControlsPreviousBtn,
		quicksettings.MediaControlsNextBtn,
	} {
		if err := res.ui.WaitUntilExists(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to find UI node on the media controls")
		}
	}

	return uiauto.Combine("enter subpage and verify media buttons' functionality",
		quicksettings.NavigateToMediaControlsSubpage(res.tconn, title),
		verifyDetailPauseButton(res.ui, player),
		verifyDetailPlayButton(res.ui, player),
		verifyDetailForwardButton(res.ui, player),
		verifyDetailBackwardButton(res.ui, player),
	)(ctx)
}

// verifyImageOnMediaControls verifies the image view is on media controls.
func verifyImageOnMediaControls(ctx context.Context, ui *uiauto.Context) error {
	// Image view is not found in the ui tree.
	// Therefore, need to verify its existence by the location of its ancestor and sibling.
	podInfo, err := ui.Info(ctx, quicksettings.MediaControlsPod)
	if err != nil {
		return errors.Wrap(err, "failed to obtain media controls pod's information")
	}
	testing.ContextLog(ctx, "Media controls pod location: ", podInfo.Location)

	// There are two subviews under quicksettings.MediaControlsPod.
	// The first one is label area with song title and singer name.
	// The other one is button area with play/pause, previous track, next track button.
	// The first one is the target sibling.
	labelView := nodewith.HasClass("View").Ancestor(quicksettings.MediaControlsPod).First()
	labelViewInfo, err := ui.Info(ctx, labelView)
	if err != nil {
		return errors.Wrap(err, "failed to obtain label view's information")
	}
	testing.ContextLog(ctx, "Label view location: ", labelViewInfo.Location)

	// If the image view exists, there will be a square area for image view,
	// whose size would be its ancestor's height x height.
	// Therefore, if the image view exists,
	// the target sibling's left margin should be greater than or equal to the image view's right margin.
	imageViewExist := podInfo.Location.Left+podInfo.Location.Height <= labelViewInfo.Location.Left

	if !imageViewExist {
		return errors.New("failed to find image view")
	}
	return nil
}

// verifyDetailPauseButton verifies if the pause button in the detailed media controls view works well.
// After clicking the pause button in the detailed media controls view,
// check if the media source on the web page is paused.
func verifyDetailPauseButton(ui *uiauto.Context, player player) uiauto.Action {
	return uiauto.Combine("pause media source by media controls",
		ui.LeftClick(quicksettings.MediaControlsDetailPauseBtn),
		player.WaitUntilPaused,
	)
}

// verifyDetailPlayButton verifies if the play button in the detailed media controls view works well.
// After clicking the play button in the detailed media controls view,
// check if the media source on the web page is playing.
func verifyDetailPlayButton(ui *uiauto.Context, player player) uiauto.Action {
	return uiauto.Combine("play media source by media controls",
		ui.LeftClick(quicksettings.MediaControlsDetailPlayBtn),
		player.WaitUntilPlaying,
	)
}

const defaultTimeout = 3 * time.Second

// verifyDetailForwardButton verifies if the forward button in the detailed media controls view works well.
// After clicking the forward button in the detailed media controls view,
// check if the media source on the web page is forward.
func verifyDetailForwardButton(ui *uiauto.Context, player player) uiauto.Action {
	return func(ctx context.Context) error {
		previousTime, err := player.RetrieveCurrentTime(ctx)
		if err != nil {
			return err
		}

		if err := ui.LeftClick(quicksettings.MediaControlsDetailForwardBtn)(ctx); err != nil {
			return errors.Wrap(err, "failed to click forward button")
		}

		testing.ContextLog(ctx, "Media control 'forward' clicked")

		const defaultForwardTime = 5 * time.Second
		// The streaming media might be buffering so that the timecode is not updated to the player bar instantly.
		// Use polling to fetch the timecode until current time is forward.
		return testing.Poll(ctx, func(ctx context.Context) error {
			currentTime, err := player.RetrieveCurrentTime(ctx)
			if err != nil {
				return err
			}
			if currentTime-previousTime > defaultForwardTime {
				testing.ContextLog(ctx, "Media control 'forward' verified")
				return nil
			}
			return errors.Errorf("the music is not forwards, previous time: %s, current time: %s", previousTime, currentTime)
		}, &testing.PollOptions{Timeout: defaultTimeout})
	}
}

// verifyDetailBackwardButton verifies if the backward button in the detailed media controls view works well.
// After clicking the backward button in the detailed media controls view,
// check if the media source on the web page is backward.
func verifyDetailBackwardButton(ui *uiauto.Context, player player) uiauto.Action {
	return func(ctx context.Context) error {
		previousTime, err := player.RetrieveCurrentTime(ctx)
		if err != nil {
			return err
		}

		if err := ui.LeftClick(quicksettings.MediaControlsDetailBackwardBtn)(ctx); err != nil {
			return errors.Wrap(err, "failed to click backward button")
		}

		testing.ContextLog(ctx, "Media control 'backward' clicked")

		// The streaming media might be buffering so that the timecode is not updated to the player bar instantly.
		// Use polling to fetch the timecode, and set polling timeout to 5 seconds,
		// which is the backward seconds after clicking backward button.
		return testing.Poll(ctx, func(ctx context.Context) error {
			currentTime, err := player.RetrieveCurrentTime(ctx)
			if err != nil {
				return err
			}
			// To verify backward button works as expected,
			// it's enough to check by the relationship between current time and preivous time.
			// Ignore the verification on default backward time (5 seconds).
			if currentTime < previousTime {
				testing.ContextLog(ctx, "Media control 'backward' verified")
				return nil
			}
			return errors.Errorf("the music is not backwards, previous time: %s, current time: %s", previousTime, currentTime)
		}, &testing.PollOptions{Timeout: defaultTimeout})
	}
}
