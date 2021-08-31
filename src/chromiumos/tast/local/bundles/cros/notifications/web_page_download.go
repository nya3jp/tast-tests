// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
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
	cr    *chrome.Chrome
	tconn *chrome.TestConn
	ui    *uiauto.Context
	kb    *input.KeyboardEventWriter
}

const (
	samplelibPrefix = "https://samplelib.com/sample-"
	jpgFileURL      = samplelibPrefix + "jpeg.html"
	pngFileURL      = samplelibPrefix + "png.html"
	gifFileURL      = samplelibPrefix + "gif.html"
	mp4FileURL      = samplelibPrefix + "mp4.html"
	webmFileURL     = samplelibPrefix + "webm.html"
	mp3FileURL      = samplelibPrefix + "mp3.html"
	wavFileURL      = samplelibPrefix + "wav.html"

	bigFileURL        = "https://speed.hetzner.de"
	bigFileName       = "10GB.bin"
	maliciousFileURL  = "http://testsafebrowsing.appspot.com/chrome"
	suspiciousFileURL = "https://crxextractor.com"

	warningLink = "https://chrome.google.com/webstore/detail/adblock-plus-free-ad-bloc/cfhdojbkjhnklbpkdaibdccddilifddb"

	defaultExpectedNotification = "Download complete"
	notificationTimeout         = time.Minute
	videoDownloadTimeout        = 3 * time.Minute // Video file requires more time to finish download.
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebPageDownload,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check if downloading files will show notifications",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      videoDownloadTimeout + 2*time.Minute,
		Params: []testing.Param{
			{
				Name: "big_file_disable",
				Val: webPageDownloadParam{
					extraFlag: disableProgressNotification,
					tests: []webPageDownloadTest{
						newBigFileTest(bigFileURL, bigFileName, "Downloading "+bigFileName, false),
					},
				},
			}, {
				Name: "big_file_enable",
				Val: webPageDownloadParam{
					extraFlag: enableProgressNotification,
					tests: []webPageDownloadTest{
						newBigFileTest(bigFileURL, bigFileName, "Downloading "+bigFileName, true),
					},
				},
			}, {
				Name: "normal_file",
				Val: webPageDownloadParam{
					tests: []webPageDownloadTest{
						newNormalFileTest("mp3 file", mp3FileURL)},
				},
			}, {
				Name: "malicious_file",
				Val: webPageDownloadParam{
					tests: []webPageDownloadTest{
						newMaliciousFileTest(maliciousFileURL, "Dangerous download blocked")},
				},
			}, {
				Name: "suspicious_file",
				Val: webPageDownloadParam{
					tests: []webPageDownloadTest{
						newSuspiciousFileTest(suspiciousFileURL, "Confirm download"),
					},
				},
			}, {
				Name: "image_file",
				Val: webPageDownloadParam{
					extraFlag: enableProgressNotification,
					tests: []webPageDownloadTest{
						newImageFileTest("jpg file", jpgFileURL),
						newImageFileTest("png file", pngFileURL),
						newImageFileTest("gif file", gifFileURL),
					},
				},
			}, {
				Name: "audio_video_file",
				Val: webPageDownloadParam{
					extraFlag: enableProgressNotification,
					tests: []webPageDownloadTest{
						newAudioVideoFileTest("mp3 file", mp3FileURL),
						newAudioVideoFileTest("wav file", wavFileURL),
						newAudioVideoFileTest("mp4 file", mp4FileURL),
						newAudioVideoFileTest("webm file", webmFileURL),
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

	resources := &webPageDownloadTestResource{
		cr:    cr,
		tconn: tconn,
		ui:    uiauto.New(tconn),
		kb:    kb,
	}

	cleanupCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, test := range testParam.tests {
		func() {
			if err := test.openPage(resources)(ctx); err != nil {
				s.Fatalf("Failed to open page %q to download sample: %v", test.getURL(), err)
			}
			defer func(ctx context.Context) {
				faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_page_ui_dump", s.TestName()))
				test.closePage()(ctx)
			}(cleanupCtx)

			if err := test.downloadSample(resources)(ctx); err != nil {
				s.Fatal("Failed to download sample: ", err)
			}
			defer test.removeSample()

			if err := test.verifyNotification(resources)(ctx); err != nil {
				s.Fatalf("Failed to verify notification on page %q: %v", test.getURL(), err)
			}
			defer func(ctx context.Context) {
				faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_notification_ui_dump", s.TestName()))
				if err := test.clearNotification(resources)(ctx); err != nil {
					// Notifications need to be clear properly or other tests might fail.
					s.Fatal("Failed to clear notification: ", err)
				}
			}(cleanupCtx)
		}()
	}
}

type webPageDownloadTest interface {
	openPage(res *webPageDownloadTestResource) action.Action
	closePage() action.Action

	downloadSample(res *webPageDownloadTestResource) action.Action
	removeSample() error

	verifyNotification(res *webPageDownloadTestResource) action.Action
	clearNotification(res *webPageDownloadTestResource) action.Action

	getDescription() string
	getURL() string
}

type webPageDownloadTestSample struct {
	progressNotificationEnabled bool
	expectedNotification        string

	description string
	url         string
	conn        *chrome.Conn
}

func (p *webPageDownloadTestSample) removeSample() error {
	files, err := filepath.Glob(filepath.Join(filesapp.DownloadPath, "sample-*"))
	if err != nil {
		return errors.Wrapf(err, "the pattern %q is malformed", "sample-*")
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return errors.Wrap(err, "failed to delete file")
		}
	}
	return nil
}

func (p *webPageDownloadTestSample) openPage(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) (retErr error) {
		var err error
		if p.conn, err = res.cr.NewConn(ctx, p.url); err != nil {
			return errors.Wrapf(err, "failed to open %s page", p.url)
		}
		defer func() {
			if retErr != nil {
				p.closePage()(ctx)
			}
		}()

		if err := webutil.WaitForQuiescence(ctx, p.conn, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for page to achieve quiescence")
		}

		return nil
	}
}

func (p *webPageDownloadTestSample) clearNotification(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		if err := ash.CloseNotifications(ctx, res.tconn); err != nil {
			return errors.Wrap(err, "failed to close all notifications")
		}
		return nil
	}
}

func (p *webPageDownloadTestSample) closePage() action.Action {
	return func(ctx context.Context) error {
		p.conn.CloseTarget(ctx)
		p.conn.Close()
		p.conn = nil
		return nil
	}
}

func (p *webPageDownloadTestSample) getURL() string         { return p.url }
func (p *webPageDownloadTestSample) getDescription() string { return p.description }

type bigFileTest struct {
	*webPageDownloadTestSample
	fileName string
}

func newBigFileTest(url, fileName, expectedNotification string, progressNotificationEnabled bool) *bigFileTest {
	return &bigFileTest{
		webPageDownloadTestSample: &webPageDownloadTestSample{
			description:                 "big file",
			expectedNotification:        expectedNotification,
			url:                         url,
			progressNotificationEnabled: progressNotificationEnabled,
		},
		fileName: fileName,
	}
}

func (bf *bigFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
	return res.ui.LeftClick(nodewith.Name(bf.fileName).Role(role.Link))
}

func (bf *bigFileTest) verifyNotification(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		if !bf.progressNotificationEnabled {
			if err := res.ui.EnsureGoneFor(nodewith.Name(bf.expectedNotification).HasClass("Label"), 10*time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to ensure notification doesn't exist")
			}
		} else {
			if _, err := ash.WaitForNotification(ctx, res.tconn, notificationTimeout, ash.WaitTitle(bf.expectedNotification)); err != nil {
				return errors.Wrapf(err, "failed to wait for notification within %v", notificationTimeout)
			}
		}

		downloadSection := holdingspace.FindDownloadChip()
		// Node of play/pause can't be distinguished from cancel in ui_tree, need to add First() or Nth().
		downloadCtrlBtn := nodewith.ClassName("ImageButton").Role(role.Button).Ancestor(downloadSection)

		return uiauto.Combine("resume, pause and cancel downloading",
			res.ui.LeftClick(holdingspace.FindTray().Role(role.Button)),
			res.ui.MouseMoveTo(downloadSection, 0),
			res.ui.LeftClick(downloadCtrlBtn.First()),
			res.ui.WaitUntilExists(downloadSection.Name("Download paused "+bf.fileName)),
			res.ui.LeftClick(downloadCtrlBtn.First()),
			res.ui.WaitUntilExists(downloadSection.Name(bf.expectedNotification)),
			res.ui.LeftClick(downloadCtrlBtn.Nth(1)),
			res.ui.WaitUntilGone(downloadSection),
		)(ctx)
	}
}

type normalFileTest struct {
	*webPageDownloadTestSample
}

func newNormalFileTest(description, url string) *normalFileTest {
	return &normalFileTest{
		&webPageDownloadTestSample{
			description:          description,
			expectedNotification: defaultExpectedNotification,
			url:                  url,
		},
	}
}

func (f *normalFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
	dlButton := nodewith.Name("Download").Role(role.Link).First()
	return uiauto.NamedAction("download video file", res.ui.LeftClick(dlButton))
}

func (f *normalFileTest) verifyNotification(res *webPageDownloadTestResource) action.Action {
	notiString := nodewith.Name("sample-3s.mp3").HasClass("Label")
	return func(ctx context.Context) error {
		if _, err := ash.WaitForNotification(ctx, res.tconn, time.Minute, ash.WaitTitle(f.expectedNotification)); err != nil {
			return errors.Wrap(err, "failed to wait for download completed notification after a minute")
		}
		if err := res.ui.WaitUntilGone(notiString)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for notification to disappear")
		}
		return nil
	}
}

type maliciousFileTest struct {
	*webPageDownloadTestSample
}

func newMaliciousFileTest(url, expectedNotification string) *maliciousFileTest {
	return &maliciousFileTest{
		&webPageDownloadTestSample{
			description:          "malicious file",
			expectedNotification: expectedNotification,
			url:                  url,
		},
	}
}

func (mf *maliciousFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
	dlButton := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.ListItem).Ancestor(nodewith.Role(role.List)).Nth(2)).Linked()
	return uiauto.NamedAction("click malicious link", res.ui.LeftClick(dlButton))
}

func (mf *maliciousFileTest) verifyNotification(res *webPageDownloadTestResource) action.Action {
	notiString := nodewith.Name("Dangerous download blocked")
	return uiauto.NamedAction("wait for dangerous download notification", res.ui.WaitUntilExists(notiString))
}

type suspiciousFileTest struct {
	*webPageDownloadTestSample
}

func newSuspiciousFileTest(url, expectedNotification string) *suspiciousFileTest {
	return &suspiciousFileTest{
		&webPageDownloadTestSample{
			description:          "suspicious file",
			expectedNotification: expectedNotification,
			url:                  url,
		},
	}
}

func (sf *suspiciousFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
	return uiauto.Combine("download chrome extension",
		res.ui.LeftClick(nodewith.Name("START FOR FREE").Role(role.Button)),
		res.ui.LeftClick(nodewith.Name("URL from Chrome WebStore").Role(role.TextField)),
		res.kb.TypeAction(warningLink),
		res.ui.LeftClick(nodewith.Name("DOWNLOAD").Role(role.InlineTextBox)),
		res.ui.LeftClick(nodewith.Name("GET .CRX").Role(role.InlineTextBox)),
	)
}

func (sf *suspiciousFileTest) verifyNotification(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		_, err := ash.WaitForNotification(ctx, res.tconn, notificationTimeout, ash.WaitTitle(sf.expectedNotification))
		return err
	}
}

type imageFileTest struct {
	*webPageDownloadTestSample
}

func newImageFileTest(description, url string) *imageFileTest {
	return &imageFileTest{
		&webPageDownloadTestSample{
			description:          description,
			expectedNotification: defaultExpectedNotification,
			url:                  url,
		},
	}
}

func (imf *imageFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
	dlButton := nodewith.Name("Download").Role(role.Link).First()
	return func(ctx context.Context) error {
		if err := res.ui.LeftClick(dlButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to download image file")
		}
		if _, err := ash.WaitForNotification(ctx, res.tconn, notificationTimeout, ash.WaitTitle(imf.expectedNotification)); err != nil {
			return errors.Wrapf(err, "failed to wait for download completed notification of image file within %v", notificationTimeout)
		}
		return nil
	}
}

func (imf *imageFileTest) verifyNotification(res *webPageDownloadTestResource) action.Action {
	notiString := nodewith.HasClass("LargeImageView")
	return uiauto.NamedAction("quick view on image file from notification", res.ui.WaitUntilExists(notiString))
}

type audioVideoFileTest struct {
	*webPageDownloadTestSample
}

func newAudioVideoFileTest(description, url string) *audioVideoFileTest {
	return &audioVideoFileTest{
		&webPageDownloadTestSample{
			description:          description,
			expectedNotification: defaultExpectedNotification,
			url:                  url,
		},
	}
}

func (avf *audioVideoFileTest) downloadSample(res *webPageDownloadTestResource) action.Action {
	dlButton := nodewith.Name("Download").Role(role.Link).First()
	return uiauto.NamedAction("download audio file", res.ui.LeftClick(dlButton))
}

func (avf *audioVideoFileTest) verifyNotification(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		if _, err := ash.WaitForNotification(ctx, res.tconn, videoDownloadTimeout, ash.WaitTitle(avf.expectedNotification)); err != nil {
			return errors.Wrapf(err, "failed to wait for download completed notification of audio file within %v", videoDownloadTimeout)
		}
		return nil
	}
}
