// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// disableProgressNotification is the flag that disable in-progress downloads notification with the productivity feature.
const disableProgressNotification = "--enable-features=HoldingSpaceInProgressNotificationSuppression"

type webPageDownloadParam struct {
	// extraFlag specifies the extra flag to be applied to start a Chrome instance.
	extraFlag string
	// tests specifies all the tests to be exercised.
	tests []webPageDownloadTest
}

// webPageDownloadTestResource holds resources for WebPageDownload.
type webPageDownloadTestResource struct {
	cr         *chrome.Chrome
	tconn      *chrome.TestConn
	ui         *uiauto.Context
	kb         *input.KeyboardEventWriter
	server     *httptest.Server
	workingDir string
	outDir     string
}

const (
	regularFileURLPrefix = "https://storage.googleapis.com/chromiumos-test-assets-public/tast/cros/"

	maliciousFileName  = "content.exe"
	suspiciousFileName = "extension_3_14_1_0.crx"
	jpgFileName        = "white_wallpaper.jpg"
	pngFileName        = "contentpreview_20210511.png"
	mp4FileName        = "720_av1_20201117.mp4"
	webmFileName       = "720_vp8_20190626.webm"
	mp3FileName        = "3s_ducking_audio_20220120.mp3"
	wavFileName        = "GLASS_20220207.wav"
	bigFileName        = "big_file_1GB_20220428.bin"

	maliciousFileURL  = "http://testsafebrowsing.appspot.com/chrome"
	suspiciousFileURL = "https://crxextractor.com"

	defaultExpectedNotification = "Download complete"
	shortTimeout                = 3 * time.Second
	downloadTimeout             = 3 * time.Minute // Some type of file needs more time to finish download, such as a video file.
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebPageDownload,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check if downloading files will show proper notification and the notification will behave as expected",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      2*time.Minute + 8*downloadTimeout, // A file will be downloaded twice in each test, there are up to 4 files in a test.
		Params: []testing.Param{
			{
				Name: "big_file_disable_progress",
				Val: webPageDownloadParam{
					extraFlag: disableProgressNotification,
					tests: []webPageDownloadTest{
						newBigFileTest(bigFileName, "Downloading "+bigFileName, false),
					},
				},
			}, {
				Name: "big_file_enable_progress",
				Val: webPageDownloadParam{
					tests: []webPageDownloadTest{
						newBigFileTest(bigFileName, "Downloading "+bigFileName, true),
					},
				},
			}, {
				Name: "malicious_file",
				Val: webPageDownloadParam{
					tests: []webPageDownloadTest{
						newMaliciousFileTest("Dangerous download blocked")},
				},
			}, {
				Name: "suspicious_file",
				Val: webPageDownloadParam{
					tests: []webPageDownloadTest{
						newSuspiciousFileTest("Confirm download"),
					},
				},
			}, {
				Name: "image_file",
				Val: webPageDownloadParam{
					tests: []webPageDownloadTest{
						newImageFileTest(jpgFileName, "jpg file"),
						newImageFileTest(pngFileName, "png file"),
					},
				},
			}, {
				Name: "audio_video_file",
				Val: webPageDownloadParam{
					tests: []webPageDownloadTest{
						newAudioVideoFileTest(mp3FileName, "mp3 file"),
						newAudioVideoFileTest(wavFileName, "wav file"),
						newAudioVideoFileTest(mp4FileName, "mp4 file"),
						newAudioVideoFileTest(webmFileName, "webm file"),
					},
				},
			},
		},
	})
}

// WebPageDownload downloads series of files and checks on each corresponding notification.
func WebPageDownload(ctx context.Context, s *testing.State) {
	cleanupCrCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testParam := s.Param().(webPageDownloadParam)

	cr, err := chrome.New(ctx, chrome.ExtraArgs(testParam.extraFlag))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCrCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	baseURL, err := url.Parse(regularFileURLPrefix)
	fullURL := baseURL
	source := map[string]string{
		bigFileName:  path.Join(baseURL.Path, "ui", bigFileName),
		jpgFileName:  path.Join(baseURL.Path, "arc", jpgFileName),
		pngFileName:  path.Join(baseURL.Path, "apps", pngFileName),
		mp4FileName:  path.Join(baseURL.Path, "video", mp4FileName),
		webmFileName: path.Join(baseURL.Path, "video", webmFileName),
		mp3FileName:  path.Join(baseURL.Path, "audio", mp3FileName),
		wavFileName:  path.Join(baseURL.Path, "audio", wavFileName),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, path := range source {
			fullURL.Path = path
			io.WriteString(w, fmt.Sprintf(`<a href=%s>%s</a><br>`, fullURL, name))
		}
		io.WriteString(w, fmt.Sprintf(`<a href=%s>%s</a><br>`, maliciousFileURL, maliciousFileName))
		io.WriteString(w, fmt.Sprintf(`<a href=%s>%s</a><br>`, suspiciousFileURL, suspiciousFileName))
	}))
	defer server.Close()

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users Downloads path: ", err)
	}

	resources := &webPageDownloadTestResource{
		cr:         cr,
		tconn:      tconn,
		ui:         uiauto.New(tconn),
		kb:         kb,
		server:     server,
		workingDir: downloadsPath,
		outDir:     s.OutDir(),
	}

	cleanupCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, test := range testParam.tests {
		func() {
			if err := test.openPage(resources)(ctx); err != nil {
				s.Fatal("Failed to open page to download sample: ", err)
			}
			defer func(ctx context.Context) {
				faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "before_close_page_ui_dump")
				test.closePage(ctx)
			}(cleanupCtx)

			for _, verify := range []uiauto.Action{
				test.verifyNotificationAutoHide(resources),   // Verify notification will autohide.
				test.verifyNotificationAndDismiss(resources), // Verify notification will appear and can be dismissed.
			} {
				func() {
					if err := test.downloadSample(resources)(ctx); err != nil {
						s.Fatal("Failed to download sample: ", err)
					}
					defer func(ctx context.Context) {
						faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "before_remove_sample_ui_dump")
						test.removeSample(resources.workingDir)
					}(cleanupCtx)

					if err := verify(ctx); err != nil {
						s.Fatal("Failed to verify notification works as expected: ", err)
					}
					defer ash.CloseNotifications(cleanupCtx, tconn)
				}()
			}
		}()
	}
}

type webPageDownloadTest interface {
	openPage(res *webPageDownloadTestResource) uiauto.Action
	closePage(ctx context.Context)

	downloadSample(res *webPageDownloadTestResource) uiauto.Action
	removeSample(workingDir string) error

	verifyNotificationAutoHide(res *webPageDownloadTestResource) uiauto.Action
	verifyNotificationAndDismiss(res *webPageDownloadTestResource) uiauto.Action
}

type webPageDownloadTestSample struct {
	webPageDownloadTest

	progressNotificationEnabled bool
	expectedNotification        string

	description string
	fileName    string
	conn        *chrome.Conn
}

func (p *webPageDownloadTestSample) openPage(res *webPageDownloadTestResource) uiauto.Action {
	return func(ctx context.Context) (retErr error) {
		var err error
		if p.conn, err = res.cr.NewConn(ctx, res.server.URL); err != nil {
			return errors.Wrapf(err, "failed to open %s page", res.server.URL)
		}
		defer func() {
			if retErr != nil {
				faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, func() bool { return retErr != nil }, res.cr, "open_page_ui_dump")
				p.closePage(ctx)
			}
		}()

		if err := webutil.WaitForRender(ctx, p.conn, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for render")
		}

		return nil
	}
}

func (p *webPageDownloadTestSample) openPageAndClick(res *webPageDownloadTestResource, dlLinkBtn *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		if err := p.openPage(res)(ctx); err != nil {
			return errors.Wrap(err, "failed to open page")
		}

		if err := res.ui.LeftClick(dlLinkBtn)(ctx); err != nil {
			return errors.Wrap(err, "failed to click download link")
		}

		// Navigates to different website after clicking on dlLinkBtn.
		// Needs to wait until the website is steady.
		if err := webutil.WaitForQuiescence(ctx, p.conn, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for page to be loaded and achieve quiescence")
		}

		return nil
	}
}

func (p *webPageDownloadTestSample) closePage(ctx context.Context) {
	if p.conn != nil {
		if err := p.conn.CloseTarget(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close target: ", err)
		}
		if err := p.conn.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close connection: ", err)
		}
		p.conn = nil
	}
}

func (p *webPageDownloadTestSample) downloadSample(res *webPageDownloadTestResource) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("download %s", p.fileName),
		res.ui.RightClick(nodewith.Name(p.fileName).Role(role.Link)),
		res.ui.LeftClick(nodewith.Name("Save link as…").HasClass("MenuItemView").Role(role.MenuItem)),
		res.ui.LeftClick(nodewith.Name("Save").Role(role.Button)),
		// Make the mouse hover out of the notification, or the notification won't be able to autohide.
		mouse.Move(res.tconn, coords.Point{X: 0, Y: 0}, time.Millisecond),
	)
}

func (p *webPageDownloadTestSample) removeSample(workingDir string) error {
	return os.Remove(filepath.Join(workingDir, p.fileName))
}

func (p *webPageDownloadTestSample) verifyNotificationAutoHide(res *webPageDownloadTestResource) uiauto.Action {
	return func(ctx context.Context) error {
		if _, err := ash.WaitForNotification(ctx, res.tconn, downloadTimeout, ash.WaitTitle(p.expectedNotification)); err != nil {
			return errors.Wrap(err, "failed to wait for download notification")
		}

		ts := time.Now()
		if err := res.ui.WaitUntilGone(nodewith.Name(p.expectedNotification))(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for notification to disappear")
		}
		goneDuration := time.Since(ts)

		// The notification must pops up and auto hides in 6s.
		// Two extra seconds is allowed for performance and latency concerns.
		if goneDuration > 8*time.Second {
			return errors.Errorf("failed to verify notification autohide, want:6s, get:%ss", goneDuration)
		}
		return nil
	}
}

func (p *webPageDownloadTestSample) verifyNotificationAndDismiss(res *webPageDownloadTestResource) uiauto.Action {
	return func(ctx context.Context) error {
		_, err := ash.WaitForNotification(ctx, res.tconn, downloadTimeout, ash.WaitTitle(p.expectedNotification))
		if err != nil {
			return errors.Wrapf(err, "failed to wait for %s download notification", p.description)
		}
		if err := res.ui.LeftClick(nodewith.Name("Notification close").Role(role.Button))(ctx); err != nil {
			return errors.Wrap(err, "failed to close notification")
		}
		return ash.WaitUntilNotificationGone(ctx, res.tconn, shortTimeout, ash.WaitTitle(p.expectedNotification))
	}
}

func (p *webPageDownloadTestSample) getDescription() string { return p.description }

type bigFileTest struct{ *webPageDownloadTestSample }

func newBigFileTest(fileName, expectedNotification string, progressNotificationEnabled bool) *bigFileTest {
	return &bigFileTest{
		webPageDownloadTestSample: &webPageDownloadTestSample{
			description:                 "big file",
			fileName:                    fileName,
			expectedNotification:        expectedNotification,
			progressNotificationEnabled: progressNotificationEnabled,
		},
	}
}

func (bf *bigFileTest) verifyNotificationAutoHide(res *webPageDownloadTestResource) uiauto.Action {
	// This test won't examine if the notification will auto hide because a big file is unlikely to finish downloading.
	// Terminate the download process by canceling it to proceed other tests.
	return uiauto.Combine("cancel downloading and verify the notification doesn't exist",
		cancelDownloading(res),
		// The notification should be gone immediately after download is canceled.
		res.ui.WithTimeout(2*time.Second).WaitUntilGone(nodewith.Name(bf.expectedNotification)),
	)
}

func (bf *bigFileTest) verifyNotificationAndDismiss(res *webPageDownloadTestResource) uiauto.Action {
	if !bf.progressNotificationEnabled {
		return uiauto.Combine("verify the noficiation doesn't exist and cancel downloading",
			// Expecting the notification not appear at all when the HoldingSpaceInProgressNotification is not enabled.
			res.ui.EnsureGoneFor(nodewith.Name(bf.expectedNotification).HasClass("Label"), 5*time.Second),
			// A big file is unlikely to finish downloading within a couple minutes, so cancel it to proceed with other tests.
			cancelDownloading(res),
		)
	}
	return uiauto.Combine("dismiss the notification and cancel downloading",
		// Dismiss the notification and verify it will be gone.
		bf.webPageDownloadTestSample.verifyNotificationAndDismiss(res),
		// A big file is unlikely to finish downloading within a couple minutes, so cancel it to proceed with other tests.
		cancelDownloading(res),
		// The notification will be dismissed by `verifyNotificationAndDismiss` above, no need to verify it is gone after cancellation.
	)
}

type maliciousFileTest struct{ *webPageDownloadTestSample }

func newMaliciousFileTest(expectedNotification string) *maliciousFileTest {
	return &maliciousFileTest{
		webPageDownloadTestSample: &webPageDownloadTestSample{
			description:          "malicious file",
			fileName:             maliciousFileName,
			expectedNotification: expectedNotification,
		},
	}
}

func (mf *maliciousFileTest) openPage(res *webPageDownloadTestResource) uiauto.Action {
	dlLinkBtn := nodewith.Name(mf.fileName).Role(role.Link)
	return mf.webPageDownloadTestSample.openPageAndClick(res, dlLinkBtn)
}

func (mf *maliciousFileTest) downloadSample(res *webPageDownloadTestResource) uiauto.Action {
	rootWindow := nodewith.Ancestor(nodewith.Role(role.Window).HasClass("BrowserRootView"))
	list := nodewith.Ancestor(rootWindow.Role(role.List))
	listItem := list.Role(role.Link)
	const expectedLinkCnt = 22

	return func(ctx context.Context) error {
		// All links are identical on the UI tree.
		// Expected 22 nodes in total and the third one is the target.
		linkNode, err := res.ui.NodesInfo(ctx, listItem)
		if err != nil {
			return errors.Wrap(err, "failed to get link nodes' info")
		} else if len(linkNode) != expectedLinkCnt {
			return errors.Errorf("failed to have %d links", expectedLinkCnt)
		}

		if err := res.ui.LeftClick(listItem.Nth(2))(ctx); err != nil {
			return errors.New("failed to click the link to download malicious file")
		}

		return nil
	}
}

type suspiciousFileTest struct{ *webPageDownloadTestSample }

func newSuspiciousFileTest(expectedNotification string) *suspiciousFileTest {
	return &suspiciousFileTest{
		&webPageDownloadTestSample{
			description:          "suspicious file",
			fileName:             suspiciousFileName,
			expectedNotification: expectedNotification,
		},
	}
}

func (sf *suspiciousFileTest) openPage(res *webPageDownloadTestResource) uiauto.Action {
	dlLinkBtn := nodewith.Name(sf.fileName).Role(role.Link)
	return sf.webPageDownloadTestSample.openPageAndClick(res, dlLinkBtn)

}

func (sf *suspiciousFileTest) downloadSample(res *webPageDownloadTestResource) uiauto.Action {
	const cwsExtensionURL = "https://chrome.google.com/webstore/detail/adblock-plus-free-ad-bloc/cfhdojbkjhnklbpkdaibdccddilifddb"
	startForFree := nodewith.Name("START FOR FREE").Role(role.Button)
	return uiauto.Combine("download chrome extension",
		// Wait until the announcement disappear. The announcement may cover the startForFree button.
		res.ui.WaitUntilGone(nodewith.Name("Switch between dark and light").Role(role.StaticText).HasClass("Label")),
		uiauto.IfSuccessThen(res.ui.WaitUntilExists(startForFree), res.ui.LeftClick(startForFree)),
		res.ui.LeftClick(nodewith.Name("URL from Chrome WebStore").Role(role.TextField)),
		res.kb.TypeAction(cwsExtensionURL),
		res.ui.LeftClick(nodewith.Name("DOWNLOAD").Role(role.InlineTextBox)),
		res.ui.LeftClick(nodewith.Name("GET .CRX").Role(role.InlineTextBox)),
	)
}

type imageFileTest struct{ *webPageDownloadTestSample }

func newImageFileTest(fileName, description string) *imageFileTest {
	return &imageFileTest{
		webPageDownloadTestSample: &webPageDownloadTestSample{
			description:          description,
			fileName:             fileName,
			expectedNotification: defaultExpectedNotification,
		},
	}
}

func (imf *imageFileTest) verifyNotificationAndDismiss(res *webPageDownloadTestResource) uiauto.Action {
	notificationImg := nodewith.HasClass("LargeImageView")
	return func(ctx context.Context) error {
		if err := res.ui.WaitUntilExists(notificationImg)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for quick view on image file from notification")
		}
		if err := res.ui.LeftClick(nodewith.Name("Notification close").Role(role.Button))(ctx); err != nil {
			return errors.Wrap(err, "failed to close notification")
		}
		return res.ui.WithTimeout(shortTimeout).WaitUntilGone(notificationImg)(ctx)
	}
}

type audioVideoFileTest struct{ *webPageDownloadTestSample }

func newAudioVideoFileTest(fileName, description string) *audioVideoFileTest {
	return &audioVideoFileTest{
		webPageDownloadTestSample: &webPageDownloadTestSample{
			description:          description,
			fileName:             fileName,
			expectedNotification: defaultExpectedNotification,
		},
	}
}

func cancelDownloading(res *webPageDownloadTestResource) uiauto.Action {
	return func(ctx context.Context) (retErr error) {
		cleanupCancelCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		conn, err := res.cr.NewConn(ctx, "chrome://downloads")
		if err != nil {
			return errors.Wrap(err, "failed to open download page")
		}
		defer func(ctx context.Context) {
			faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, func() bool { return retErr != nil }, res.cr, "cancel_downloading_ui_dump")
			conn.CloseTarget(ctx)
			conn.Close()
		}(cleanupCancelCtx)

		download := nodewith.Ancestor(nodewith.Name("Downloads").Role(role.RootWebArea))
		return uiauto.Combine("cancel downloading",
			res.ui.LeftClick(download.Name("Cancel").Role(role.Cell)),
			// Remove the download process from the list.
			res.ui.LeftClick(download.NameStartingWith("Remove").Role(role.Cell)),
			res.ui.WaitUntilGone(download.Name("Canceled").Role(role.StaticText)),
		)(ctx)
	}
}
