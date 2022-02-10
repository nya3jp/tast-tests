// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaSessionAPI,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify the control buttons exist and there should be a space for artwork if the audio has artwork",
		Contacts: []string{
			"cj.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"cros-status-area-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"media_session_api.html", "Lenna.png", "five_minute_audio_20211116.mp3"},
		Params: []testing.Param{
			{
				Fixture: "chromeLoggedIn",
				Val:     browser.TypeAsh,
			}, {
				Name:              "lacros",
				Fixture:           "lacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               browser.TypeLacros,
			},
		},
		Timeout: 3 * time.Minute,
	})
}

// MediaSessionAPI verifies the control buttons exist and there should be a space for artwork if the audio has artwork.
func MediaSessionAPI(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	browserType := s.Param().(browser.Type)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard: ", err)
	}
	defer kb.Close()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// TODO(crbug.com/1259615): This should be part of the fixture.
	// Setup browser based on the chrome type.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	conn, err := br.NewConn(ctx, filepath.Join(server.URL, "media_session_api.html"))
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump")
		conn.CloseTarget(ctx)
		conn.Close()
	}(cleanupCtx)

	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
		s.Fatal("Failed to wait until the page is stable: ", err)
	}

	browserFinder := nodewith.Ancestor(nodewith.Role(role.Window).HasClass("BrowserFrame").NameContaining("MediaSessionAPI"))
	if browserType == browser.TypeLacros {
		classNameRegexp := regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)
		browserFinder = nodewith.Ancestor(nodewith.Role(role.Window).ClassNameRegex(classNameRegexp).NameContaining("MediaSessionAPI"))
	}

	playButton := browserFinder.Name("play").Role(role.Button)
	if err := uiauto.Combine("play the audio",
		ui.LeftClick(playButton),
		// It might take a longer time to wait until the button show up.
		ui.WithTimeout(time.Minute).WaitUntilExists(quicksettings.PinnedMediaControls),
		ui.LeftClick(quicksettings.PinnedMediaControls),
		ui.WaitUntilExists(quicksettings.MediaControlsDialog),
	)(ctx); err != nil {
		s.Fatal("Failed to complete all actions: ", err)
	}

	for _, test := range []mediaSessionAPITest{
		{
			ui:         ui,
			hasArtwork: true,
		}, {
			ui:         ui,
			hasArtwork: false,
		},
	} {
		subtest := func(ctx context.Context, s *testing.State) {
			cleanupSubCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			if err := conn.Call(ctx, &test.audioName, "getTitleWithArtwork", test.hasArtwork); err != nil {
				s.Fatal("Failed to get the title of audio: ", err)
			}
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubCtx, s.OutDir(), s.HasError, cr, test.audioName)

			s.Logf("Switching to target audio: %q", test.audioName)
			if err := test.switchToTargetAudio(ctx); err != nil {
				s.Fatal("Failed to switch to target audio: ", err)
			}

			s.Log("Verifing media controls buttons exist")
			if err := test.verifyMediaControlNodes(ctx); err != nil {
				s.Fatal("Failed to verify nodes in media control: ", err)
			}

			s.Log("Verifing media artwork")
			if err := test.verifyArtwork(ctx); err != nil {
				s.Fatal("Failed to verify artwork existed: ", err)
			}
		}
		if !s.Run(ctx, test.audioName, subtest) {
			s.Errorf("Failed to run subtest: %q", test.audioName)
		}
	}
}

type mediaSessionAPITest struct {
	ui         *uiauto.Context
	audioName  string
	hasArtwork bool
}

func (m *mediaSessionAPITest) switchToTargetAudio(ctx context.Context) error {
	if err := uiauto.IfSuccessThen(
		m.ui.WaitUntilGone(quicksettings.MediaControlsDialog),
		m.ui.LeftClick(quicksettings.PinnedMediaControls),
	)(ctx); err != nil {
		return err
	}

	audioLabel := nodewith.NameContaining(m.audioName).Role(role.StaticText).HasClass("Label")
	return uiauto.IfSuccessThen(
		m.ui.WaitUntilGone(audioLabel.Ancestor(quicksettings.MediaControlsDialog)),
		m.ui.RetryUntil(
			m.ui.LeftClick(nodewith.Name("Next Track").Role(role.Button).Ancestor(quicksettings.MediaControlsDialog)),
			m.ui.WithTimeout(3*time.Second).WaitUntilExists(audioLabel.Ancestor(quicksettings.MediaControlsDialog)),
		),
	)(ctx)
}

func (m *mediaSessionAPITest) verifyMediaControlNodes(ctx context.Context) error {
	audioLabel := nodewith.NameContaining(m.audioName).Role(role.StaticText).HasClass("Label")
	if err := uiauto.IfSuccessThen(
		m.ui.WaitUntilGone(audioLabel.Ancestor(quicksettings.MediaControlsDialog)),
		m.ui.LeftClick(quicksettings.PinnedMediaControls),
	)(ctx); err != nil {
		return err
	}

	for _, node := range []*nodewith.Finder{
		nodewith.Name("Pause").Role(role.ToggleButton),
		nodewith.Name("Seek Backward").Role(role.Button),
		nodewith.Name("Seek Forward").Role(role.Button),
		nodewith.Name("Previous Track").Role(role.Button),
		nodewith.Name("Next Track").Role(role.Button),
	} {
		if err := m.ui.WaitUntilExists(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to find node")
		}
	}
	return nil
}

func (m *mediaSessionAPITest) verifyArtwork(ctx context.Context) error {
	mediaListContainer := nodewith.Role(role.ListItem).HasClass("MediaNotificationViewImpl").Ancestor(quicksettings.MediaControlsDialog)
	mediaListContainerLocation, err := m.ui.Location(ctx, mediaListContainer)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of media list container")
	}
	testing.ContextLog(ctx, "Media list container location: ", mediaListContainerLocation)

	// The image view is not available on the UI tree and its existence can only be examined by other nodes.
	// These are the final details used to examine if the image view exists.
	var (
		dissmissBtnLocation     *coords.Rect
		mediaDetailViewLocation *coords.Rect
	)

	dismissBtn := nodewith.Role(role.Button).Name("Dismiss").Ancestor(mediaListContainer)
	if err := uiauto.Combine("make dismiss button visible",
		m.ui.WaitForLocation(mediaListContainer),
		m.ui.MouseMoveTo(mediaListContainer, 200*time.Millisecond),
		m.ui.WaitUntilExists(dismissBtn),
	)(ctx); err != nil {
		return err
	}

	if dissmissBtnLocation, err = m.ui.Location(ctx, dismissBtn); err != nil {
		return errors.Wrap(err, "failed to get the location of dismiss button")
	}
	testing.ContextLog(ctx, "Dissmiss button location: ", dissmissBtnLocation)

	subViews := nodewith.HasClass("View").Ancestor(mediaListContainer)
	if err := m.ui.WaitUntilExists(subViews.First())(ctx); err != nil {
		return errors.Wrap(err, "failed to find any subviews under the media control dialog")
	}
	subViewInfos, err := m.ui.NodesInfo(ctx, subViews)
	if err != nil {
		return err
	}

	var subviewNth int
	matchedCnt := 0
	testing.ContextLog(ctx, "Searching for subview has the same width as media list container")
	for nth, subViewInfo := range subViewInfos {
		// All subviews under media list container are identical on the UI tree.
		// We examine the width of a subview to further locate the target.
		if subViewInfo.Location.Width != mediaListContainerLocation.Width {
			continue
		}
		// Expecting 2 subviews have the same width as media list container:
		//    1. list header view
		//    2. list contents container view
		// Second one is the target, let the value be overwritten directly.
		subviewNth = nth
		matchedCnt++
	}

	if matchedCnt != 2 {
		return errors.Errorf("expecting 2 subviews have the same width as media list container, got: %d", matchedCnt)
	}
	listContentsContainerView := subViews.Nth(subviewNth)

	// The first subview under the list contents container view is the final target.
	mediaDetailView := nodewith.HasClass("View").First().Ancestor(listContentsContainerView)
	if mediaDetailViewLocation, err = m.ui.Location(ctx, mediaDetailView); err != nil {
		return errors.Wrap(err, "failed to get the location of media detail view")
	}
	testing.ContextLog(ctx, "Media detail view location: ", mediaDetailViewLocation)

	// The image view is not available on the UI tree.
	// The dismiss button is located at the right side of media controls dialog and right-aligned to media item.
	// Within a media item, there should be a detail view on the left and an image view on the right.
	// If the detail view's right bound is reached to the right bound of the media item, meaning the image view doesn't exist (there is no space for image view).
	hasArtwork := mediaDetailViewLocation.Right() != dissmissBtnLocation.Right()
	if hasArtwork != m.hasArtwork {
		return errors.Errorf("failed to verify media has artwork: want %t, got %t", m.hasArtwork, hasArtwork)
	}
	return nil
}
