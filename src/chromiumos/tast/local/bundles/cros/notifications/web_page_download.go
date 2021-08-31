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
	"regexp"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// enableProgressNotification is the flag that enable in-progress downloads notification with the productivity feature.
	enableProgressNotification = "--disable-features=HoldingSpaceInProgressNotificationSuppression"
	// disableProgressNotification is the flag that disable in-progress downloads notification with the productivity feature.
	disableProgressNotification = "--enable-features=HoldingSpaceInProgressNotificationSuppression"
)

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
}

const (
	regularFileURLPrefix = "https://storage.googleapis.com/chromiumos-test-assets-public/tast/cros/"

	jpgFileName  = "white_wallpaper.jpg"
	pngFileName  = "contentpreview_20210511.png"
	mp4FileName  = "720_av1_20201117.mp4"
	webmFileName = "720_vp8_20190626.webm"
	mp3FileName  = "3s_ducking_audio_20220120.mp3"
	wavFileName  = "GLASS_20220207.wav"
	bigFileName  = "big_file_1GB_20220428.bin"

	maliciousFileURL  = "http://testsafebrowsing.appspot.com/chrome"
	suspiciousFileURL = "https://crxextractor.com"

	defaultExpectedNotification = "Download complete"
	shortTimeout                = 3 * time.Second
	notificationTimeout         = time.Minute
	videoDownloadTimeout        = 3 * time.Minute // Video file requires more time to finish download.
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebPageDownload,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check if downloading files will show proper notification and the notification will behave as expected",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      videoDownloadTimeout + 10*time.Minute,
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
					extraFlag: enableProgressNotification,
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
					extraFlag: enableProgressNotification,
					tests: []webPageDownloadTest{
						newImageFileTest(jpgFileName, "jpg file"),
						newImageFileTest(pngFileName, "png file"),
					},
				},
			}, {
				Name: "audio_video_file",
				Val: webPageDownloadParam{
					extraFlag: enableProgressNotification,
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
		io.WriteString(w, fmt.Sprintf(`<a href=%s>malicious file</a><br>`, maliciousFileURL))
		io.WriteString(w, fmt.Sprintf(`<a href=%s>suspicious file</a><br>`, suspiciousFileURL))
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
				faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_page_ui_dump", s.TestName()))
				test.closePage(ctx)
			}(cleanupCtx)

			// Verify notification will autohide.
			func() {
				if err := test.downloadSample(resources)(ctx); err != nil {
					s.Fatal("Failed to download sample: ", err)
				}
				defer test.removeSample(resources.workingDir)

				if err := test.verifyNotificationAutoHide(resources)(ctx); err != nil {
					s.Fatal("Failed to verify notification auto-hide: ", err)
				}
				defer func(ctx context.Context) {
					faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_notificationautohide_ui_dump", s.TestName()))
					ash.CloseNotifications(ctx, tconn)
				}(cleanupCtx)
			}()

			// Verify notification will appear and can be dismissed.
			func() {
				if err := test.downloadSample(resources)(ctx); err != nil {
					s.Fatal("Failed to download sample: ", err)
				}
				defer test.removeSample(resources.workingDir)

				if err := test.verifyNotificationAndDismiss(resources)(ctx); err != nil {
					s.Fatal("Failed to verify closing notification: ", err)
				}
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_notification_ui_dump", s.TestName()))
			}()
		}()
	}
}

type webPageDownloadTest interface {
	openPage(res *webPageDownloadTestResource) action.Action
	closePage(ctx context.Context)

	downloadSample(res *webPageDownloadTestResource) action.Action
	removeSample(workingDir string) error

	verifyNotificationAutoHide(res *webPageDownloadTestResource) action.Action
	verifyNotificationAndDismiss(res *webPageDownloadTestResource) action.Action

	getDescription() string
}

type webPageDownloadTestSample struct {
	webPageDownloadTest

	progressNotificationEnabled bool
	expectedNotification        string

	description string
	fileName    string
	conn        *chrome.Conn
}

func (p *webPageDownloadTestSample) openPage(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) (retErr error) {
		var err error
		if p.conn, err = res.cr.NewConn(ctx, res.server.URL); err != nil {
			return errors.Wrapf(err, "failed to open %s page", res.server.URL)
		}
		defer func() {
			if retErr != nil {
				p.closePage(ctx)
			}
		}()

		if err := webutil.WaitForRender(ctx, p.conn, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for render")
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

func (p *webPageDownloadTestSample) downloadSample(res *webPageDownloadTestResource) action.Action {
	return uiauto.Combine(fmt.Sprintf("download %s", p.fileName),
		res.ui.RightClick(nodewith.Name(p.fileName).Role(role.Link)),
		res.ui.LeftClick(nodewith.Name("Save link as…").HasClass("MenuItemView").Role(role.MenuItem)),
		res.ui.LeftClick(nodewith.Name("Save").HasClass("ok primary").Role(role.Button)),
		// Make the mouse hover out of the notification, or the notification won't be able to autohide.
		mouse.Move(res.tconn, coords.Point{X: 0, Y: 0}, time.Millisecond),
	)
}

func (p *webPageDownloadTestSample) removeSample(workingDir string) error {
	return os.Remove(filepath.Join(workingDir, p.fileName))
}

func (p *webPageDownloadTestSample) verifyNotificationAutoHide(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		if _, err := ash.WaitForNotification(ctx, res.tconn, time.Minute, ash.WaitTitle(p.expectedNotification)); err != nil {
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

func (p *webPageDownloadTestSample) verifyNotificationAndDismiss(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		_, err := ash.WaitForNotification(ctx, res.tconn, notificationTimeout, ash.WaitTitle(p.expectedNotification))
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

func (bf *bigFileTest) removeSample(workingDir string) error {
	// There won't be any file to remove due to the download process of a big file won't be finished in this test.
	return nil
}

func (bf *bigFileTest) verifyNotificationAutoHide(res *webPageDownloadTestResource) action.Action {
	// The file is too big to finish download so just cancel the download.
	downloadSection := holdingspace.FindDownloadChip().Name(fmt.Sprintf("Downloading %s", bf.fileName))
	closeBtn := nodewith.ClassName("ImageButton").Role(role.Button).Ancestor(downloadSection).Nth(1)

	return uiauto.Combine("cancel downloading",
		res.ui.LeftClick(holdingspace.FindTray().Role(role.Button)),
		// Hover on the downloadSection to make the button appears.
		res.ui.MouseMoveTo(downloadSection, 0),
		res.ui.LeftClick(closeBtn),
		res.ui.WaitUntilGone(downloadSection),
	)
}

func (bf *bigFileTest) verifyNotificationAndDismiss(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		if !bf.progressNotificationEnabled {
			if err := res.ui.EnsureGoneFor(nodewith.Name(bf.expectedNotification).HasClass("Label"), 5*time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to ensure notification doesn't exist")
			}
		} else {
			if _, err := ash.WaitForNotification(ctx, res.tconn, notificationTimeout, ash.WaitTitle(bf.expectedNotification)); err != nil {
				return errors.Wrapf(err, "failed to wait for notification within %v", notificationTimeout)
			}
		}

		root := holdingspace.FindDownloadChip()
		downloadSection := root.NameRegex(regexp.MustCompile(`Download.*`))
		downloading := root.NameRegex(regexp.MustCompile(`Downloading .*`))
		downloadPaused := root.NameRegex(regexp.MustCompile(`Download paused .*`))
		controlBtns := nodewith.ClassName("ImageButton").Role(role.Button)
		pauseBtn := controlBtns.Ancestor(downloading).First()
		continueBtn := controlBtns.Ancestor(downloadPaused).First()
		cancelBtn := controlBtns.Ancestor(downloadSection).Nth(1) // This cancel button is identical with the pause/continue button.

		return uiauto.Combine("resume, pause and cancel downloading",
			res.ui.LeftClick(holdingspace.FindTray().Role(role.Button)),
			// Hover on the downloadSection to make the button appears.
			res.ui.MouseMoveTo(downloading, 0),
			res.ui.LeftClick(pauseBtn),
			res.ui.WaitUntilExists(downloadPaused),
			res.ui.LeftClick(continueBtn),
			res.ui.WaitUntilExists(downloading.Name(bf.expectedNotification)),
			res.ui.LeftClick(cancelBtn),
			res.ui.WaitUntilGone(downloading),
			res.ui.WaitUntilGone(downloadPaused),
		)(ctx)
	}
}

type maliciousFileTest struct{ *webPageDownloadTestSample }

func newMaliciousFileTest(expectedNotification string) *maliciousFileTest {
	return &maliciousFileTest{
		webPageDownloadTestSample: &webPageDownloadTestSample{
			description:          "malicious file",
			fileName:             "malicious file",
			expectedNotification: expectedNotification,
		},
	}
}

func (mf *maliciousFileTest) openPage(res *webPageDownloadTestResource) action.Action {
	dlLinkBtn := nodewith.Name(mf.fileName).Role(role.Link)
	return func(ctx context.Context) error {
		if err := mf.webPageDownloadTestSample.openPage(res)(ctx); err != nil {
			return errors.Wrap(err, "failed to open page")
		}

		if err := res.ui.LeftClick(dlLinkBtn)(ctx); err != nil {
			return errors.Wrap(err, "failed to click download link")
		}

		if err := webutil.WaitForQuiescence(ctx, mf.conn, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for page to be loaded and achieve quiescence")
		}

		return nil
	}
}

func (mf *maliciousFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
	dlButton := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.ListItem).Ancestor(nodewith.Role(role.List)).Nth(2)).Linked()
	return res.ui.LeftClick(dlButton)
}

func (mf *maliciousFileTest) removeSample(workingDir string) error {
	// There won't be any file to remove due to the download process of a malicious file won't be finished in this test.
	return nil
}

type suspiciousFileTest struct{ *webPageDownloadTestSample }

func newSuspiciousFileTest(expectedNotification string) *suspiciousFileTest {
	return &suspiciousFileTest{
		&webPageDownloadTestSample{
			description:          "suspicious file",
			fileName:             "suspicious file",
			expectedNotification: expectedNotification,
		},
	}
}

func (sf *suspiciousFileTest) openPage(res *webPageDownloadTestResource) action.Action {
	dlLinkBtn := nodewith.Name(sf.fileName).Role(role.Link)
	return func(ctx context.Context) error {
		if err := sf.webPageDownloadTestSample.openPage(res)(ctx); err != nil {
			return errors.Wrap(err, "failed to open page")
		}

		if err := res.ui.LeftClick(dlLinkBtn)(ctx); err != nil {
			return errors.Wrap(err, "failed to click download link")
		}

		if err := webutil.WaitForQuiescence(ctx, sf.conn, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for page to be loaded and achieve quiescence")
		}

		return nil
	}
}

func (sf *suspiciousFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
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

func (sf *suspiciousFileTest) removeSample(workingDir string) error {
	// There won't be any file to remove due to the download process of a suspicious file won't be finished in this test.
	return nil
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

func (imf *imageFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		if err := imf.webPageDownloadTestSample.downloadSample(res)(ctx); err != nil {
			return errors.Wrap(err, "failed to download image file")
		}
		if _, err := ash.WaitForNotification(ctx, res.tconn, notificationTimeout, ash.WaitTitle(imf.expectedNotification)); err != nil {
			return errors.Wrapf(err, "failed to wait for download completed notification of image file within %v", notificationTimeout)
		}
		return nil
	}
}

func (imf *imageFileTest) verifyNotificationAndDismiss(res *webPageDownloadTestResource) action.Action {
	notiString := nodewith.HasClass("LargeImageView")
	return func(ctx context.Context) error {
		if err := res.ui.WaitUntilExists(notiString)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for quick view on image file from notification")
		}
		if err := res.ui.LeftClick(nodewith.Name("Notification close").Role(role.Button))(ctx); err != nil {
			return errors.Wrap(err, "failed to close notification")
		}
		return res.ui.WithTimeout(shortTimeout).WaitUntilGone(notiString)(ctx)
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
