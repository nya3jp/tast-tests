// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/ambient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/youtube"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetScreensaver,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting screensaver in the personalization hub app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ambient.username", "ambient.password"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "personalizationWithGaiaLogin",
		Params: []testing.Param{
			{
				Name: "google_photos",
				Val: ambient.TestParams{
					TopicSource:            ambient.GooglePhotos,
					Theme:                  ambient.SlideShow,
					AnimationPlaybackSpeed: ambient.SlideShowDefaultPlaybackSpeed,
					AnimationStartTimeout:  ambient.AmbientStartSlideShowDefaultTimeout,
				},
			},
			{
				Name: "art_gallery",
				Val: ambient.TestParams{
					TopicSource:            ambient.ArtGallery,
					Theme:                  ambient.SlideShow,
					AnimationPlaybackSpeed: ambient.SlideShowDefaultPlaybackSpeed,
					AnimationStartTimeout:  ambient.AmbientStartSlideShowDefaultTimeout,
				},
			},
			// For animated themes:
			// * Their image handling is agnostic to the topic source, so it would be
			//   a waste of test time/resources to test all topic sources for each
			//   theme.
			// * ArtGallery is chosen as the topic source since some of the photos may
			//   have attribution text (whereas personal photos do not). This gives
			//   the test better coverage since attribution text handling is not
			//   trivial.
			{
				Name: "feel_the_breeze",
				Val: ambient.TestParams{
					TopicSource:            ambient.ArtGallery,
					Theme:                  ambient.FeelTheBreeze,
					AnimationPlaybackSpeed: ambient.AnimationDefaultPlaybackSpeed,
					AnimationStartTimeout:  ambient.AmbientStartAnimationDefaultTimeout,
				},
			},
			{
				Name: "float_on_by",
				Val: ambient.TestParams{
					TopicSource:            ambient.ArtGallery,
					Theme:                  ambient.FloatOnBy,
					AnimationPlaybackSpeed: ambient.AnimationDefaultPlaybackSpeed,
					AnimationStartTimeout:  ambient.AmbientStartAnimationDefaultTimeout,
				},
			},
		},
	})
}

func SetScreensaver(ctx context.Context, s *testing.State) {
	testParams := s.Param().(ambient.TestParams)
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Force Chrome to be in clamshell mode to make sure it's possible to close
	// the personalization hub.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := ambient.OpenAmbientSubpage(ctx, ui); err != nil {
		s.Fatal("Failed to open Ambient Subpage: ", err)
	}

	if err := ambient.EnableAmbientMode(ctx, ui); err != nil {
		s.Fatal("Failed to enable ambient mode: ", err)
	}

	if err := prepareScreensaver(ctx, tconn, ui, testParams); err != nil {
		s.Fatalf("Failed to prepare %v/%v screensaver: %v", testParams.TopicSource, testParams.Theme, err)
	}

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	var uiHandler cuj.UIActionHandler
	if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	// Open up an arbitrary Youtube video to test "media string". The name of
	// the media playing should be displayed in the screensaver.
	const extendedDisplay = false
	videoApp := youtube.NewYtWeb(cr.Browser(), tconn, kb, ambient.TestVideoSrc, extendedDisplay, ui, uiHandler)
	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", ambient.TestVideoSrc.URL, err)
	}
	defer videoApp.Close(cleanupCtx)

	if err := ambient.TestLockScreenIdle(ctx, cr, tconn, ui, testParams.AnimationStartTimeout); err != nil {
		s.Fatal("Failed to start ambient mode: ", err)
	}

	if err := ambient.UnlockScreen(ctx, tconn, s.RequiredVar("ambient.username"), s.RequiredVar("ambient.password")); err != nil {
		s.Fatal("Failed to unlock screen: ", err)
	}
}

func prepareScreensaver(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, testParams ambient.TestParams) error {
	themeContainer := nodewith.Role(role.RadioButton).Name(testParams.Theme)
	if err := uiauto.Combine("Choose animation theme",
		ui.FocusAndWait(themeContainer),
		ui.LeftClick(themeContainer))(ctx); err != nil {
		return errors.Wrapf(err, "failed to select %v", testParams.Theme)
	}

	topicSourceContainer := nodewith.Role(role.RadioButton).NameContaining(testParams.TopicSource)
	albumsFinder := nodewith.Role(role.ListBoxOption).HasClass("album")

	if err := uiauto.Combine("Choose topic source",
		ui.FocusAndWait(topicSourceContainer),
		ui.LeftClick(topicSourceContainer),
		ui.WaitUntilExists(albumsFinder.First()))(ctx); err != nil {
		return errors.Wrapf(err, "failed to select %v", testParams.TopicSource)
	}

	albums, err := ui.NodesInfo(ctx, albumsFinder)
	if err != nil {
		return errors.Wrapf(err, "failed to find %v albums", testParams.TopicSource)
	}
	if len(albums) < 2 {
		return errors.Errorf("at least 2 %v albums expected", testParams.TopicSource)
	}

	// For animated themes, trust the default album selection. Test cases for
	// slideshow theme will verify that the default album selection is correct
	// and test custom album selection.
	if testParams.Theme == ambient.SlideShow {
		if testParams.TopicSource == ambient.GooglePhotos {
			// Select all Google Photos albums.
			for i, album := range albums {
				if strings.Contains(album.ClassName, "album-selected") {
					return errors.Errorf("Google Photos album %d should be unselected", i)
				}
				selectedAlbumNode := nodewith.HasClass("album-selected").Name(album.Name)
				if err := ui.RetryUntil(uiauto.Combine("select Google Photo album",
					ui.Gone(selectedAlbumNode),
					ui.MouseClickAtLocation(0, album.Location.CenterPoint())),
					ui.WaitUntilExists(selectedAlbumNode))(ctx); err != nil {
					return errors.Wrapf(err, "failed to select Google Photos album %d", i)
				}
			}
		} else if testParams.TopicSource == ambient.ArtGallery {
			// Turn off all but one art gallery album.
			for i, album := range albums[1:] {
				if !strings.Contains(album.ClassName, "album-selected") {
					return errors.Errorf("Art album %d should be selected", i)
				}
				selectedAlbumNode := nodewith.HasClass("album-selected").Name(album.Name)
				if err := ui.RetryUntil(uiauto.Combine("deselect Art Gallery album",
					ui.Exists(selectedAlbumNode),
					ui.MouseClickAtLocation(0, album.Location.CenterPoint())),
					ui.WaitUntilGone(selectedAlbumNode))(ctx); err != nil {
					return errors.Wrapf(err, "failed to deselect Art Gallery album %d", i)
				}
			}
		} else {
			return errors.Errorf("topicSource - %v is invalid", testParams.TopicSource)
		}
	}

	// Close Personalization Hub after ambient mode setup is finished.
	if err := personalization.ClosePersonalizationHub(ui)(ctx); err != nil {
		return errors.Wrap(err, "failed to close Personalization Hub")
	}

	if err := ambient.SetDeviceSettings(
		ctx,
		tconn,
		ambient.DeviceSettings{
			LockScreenIdle:         1 * time.Second,
			BackgroundLockScreen:   2 * time.Second,
			PhotoRefreshInterval:   1 * time.Second,
			AnimationPlaybackSpeed: testParams.AnimationPlaybackSpeed,
		},
	); err != nil {
		return errors.Wrap(err, "failed to configure ambient settings")
	}

	return nil
}
