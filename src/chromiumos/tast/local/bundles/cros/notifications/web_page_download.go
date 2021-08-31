// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

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
	maliciousFileURL  = "http://testsafebrowsing.appspot.com/chrome"
	suspiciousFileURL = "https://crxextractor.com"

	warningLink = "https://chrome.google.com/webstore/detail/adblock-plus-free-ad-bloc/cfhdojbkjhnklbpkdaibdccddilifddb"

	defaultExpectedNotification = "Download complete"
	notificationTimeout         = time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebPageDownload,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check if downloading files will show notifications",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "enable", // Enables in-progress downloads notification suppression with the productivity feature.
				Val:  false,    // Expecting downloads progress notification appear or not.
			}, {
				Name: "disable", // Disables in-progress downloads notification suppression with the productivity feature.
				Val:  true,      // Expecting downloads progress notification appear or not.
			}},
	})
}

// WebPageDownload downloads series of files and checks on each corresponding notification.
func WebPageDownload(ctx context.Context, s *testing.State) {
	cleanupCrCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	isDownloadsProgressNotificationExpected := s.Param().(bool)

	flag := "--disable-features=HoldingSpaceInProgressNotificationSuppression"
	if !isDownloadsProgressNotificationExpected {
		flag = "--enable-features=HoldingSpaceInProgressNotificationSuppression"
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(flag))
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

	for _, test := range []struct {
		description string
		samples     []downloadAndVerifyNotification
	}{
		{
			description: "pause, resume and cancel download",
			samples: []downloadAndVerifyNotification{
				newBigFile(bigFileURL, "Downloading 10GB.bin", isDownloadsProgressNotificationExpected),
			},
		}, {
			description: "notification will autohide after 6 seconds",
			samples: []downloadAndVerifyNotification{
				newNormalFile(mp3FileURL),
			},
		}, {
			description: "malicious download will show `Dangerous download blocked` in notification",
			samples: []downloadAndVerifyNotification{
				newMaliciousFile(maliciousFileURL, "Dangerous download blocked"),
			},
		}, {
			description: "suspicious download will show `Confirm download` in notification",
			samples: []downloadAndVerifyNotification{
				newSuspiciousFile(suspiciousFileURL, "Confirm download"),
			},
		}, {
			description: "downloading image will show a preview in notification",
			samples: []downloadAndVerifyNotification{
				newImageFile(jpgFileURL),
				newImageFile(pngFileURL),
				newImageFile(gifFileURL),
			},
		}, {
			description: "downloading audio or video will show a notification",
			samples: []downloadAndVerifyNotification{
				newAudioVideoFile(mp3FileURL),
				newAudioVideoFile(wavFileURL),
				newAudioVideoFile(mp4FileURL),
				newAudioVideoFile(webmFileURL),
			},
		},
	} {
		f := func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			for _, sample := range test.samples {
				if err := sample.openPage(cr)(ctx); err != nil {
					s.Fatalf("Failed to open page %q to download sample: %v", sample.getURL(), err)
				}
				defer sample.closePage()(cleanupCtx)
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "page_ui_dump")

				if err := sample.download(resources)(ctx); err != nil {
					s.Fatal("Failed to download sample: ", err)
				}
				defer sample.remove()
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "notification_ui_dump")

				if err := sample.verify(resources)(ctx); err != nil {
					s.Fatalf("Failed to verify notification on page %q: %v", sample.getURL(), err)
				}

				// Clear notification to ensure other tests won't detect an unexpected notification.
				if err := sample.clearNotification(resources.ui, resources.tconn)(ctx); err != nil {
					s.Fatal("Failed to clear notification: ", err)
				}
			}
		}

		if !s.Run(ctx, test.description, f) {
			s.Errorf("Failed to complete test: %s", test.description)
		}
	}
}

type webPageDownloadTestSample struct {
	expectedNotification  string
	url                   string
	expectedProgressShown bool
	conn                  *chrome.Conn
}

type pageControl interface {
	openPage(cr *chrome.Chrome) action.Action
	clearNotification(ui *uiauto.Context, tconn *chrome.TestConn) action.Action
	closePage() action.Action
	remove() error
	getURL() string
}

func (p *webPageDownloadTestSample) remove() error {
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

func (p *webPageDownloadTestSample) openPage(cr *chrome.Chrome) action.Action {
	return func(ctx context.Context) (retErr error) {
		var err error
		if p.conn, err = cr.NewConn(ctx, p.url); err != nil {
			return errors.Wrapf(err, "failed to open %s page", p.url)
		}
		defer func() {
			if retErr != nil {
				p.conn.CloseTarget(ctx)
				p.conn.Close()
				p.conn = nil
			}
		}()

		if err := webutil.WaitForQuiescence(ctx, p.conn, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for page to achieve quiescence")
		}

		return nil
	}
}

func (p *webPageDownloadTestSample) clearNotification(ui *uiauto.Context, tconn *chrome.TestConn) action.Action {
	return func(ctx context.Context) error {
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
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

func (p *webPageDownloadTestSample) getURL() string {
	return p.url
}

type downloadAndVerifyNotification interface {
	download(res *webPageDownloadTestResource) action.Action
	verify(res *webPageDownloadTestResource) action.Action
	pageControl
}

type bigFile struct {
	*webPageDownloadTestSample
}

func newBigFile(url, expectedNotification string, expectedProgressShown bool) *bigFile {
	return &bigFile{
		&webPageDownloadTestSample{
			expectedNotification:  expectedNotification,
			url:                   url,
			expectedProgressShown: expectedProgressShown,
		},
	}
}

func (bf *bigFile) download(res *webPageDownloadTestResource) action.Action {
	return res.ui.LeftClick(nodewith.Name("10GB.bin").Role(role.Link))
}

func (bf *bigFile) verify(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		if !bf.expectedProgressShown {
			if err := res.ui.EnsureGoneFor(nodewith.Name(bf.expectedNotification).HasClass("Label"), 10*time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to ensure notification doesn't exist")
			}
		} else {
			if _, err := ash.WaitForNotification(ctx, res.tconn, notificationTimeout, ash.WaitTitle(bf.expectedNotification)); err != nil {
				return errors.Wrapf(err, "failed to wait for notification within %v", notificationTimeout)
			}
		}

		// Open the download page.
		if err := res.kb.Accel(ctx, "Ctrl+j"); err != nil {
			return errors.Wrap(err, "failed to open download page")
		}
		// Close the download page.
		defer res.kb.AccelAction("Ctrl+w")(ctx)

		cell := nodewith.Role(role.Cell).Ancestor(nodewith.HasClass("controls"))
		return uiauto.Combine("resume, pause and cancel downloading",
			res.ui.LeftClick(cell.Name("Pause")),
			res.ui.LeftClick(cell.Name("Resume").Role(role.Button)),
			res.ui.LeftClick(cell.Name("Cancel")),
		)(ctx)
	}
}

type normalFile struct {
	*webPageDownloadTestSample
}

func newNormalFile(url string) *normalFile {
	return &normalFile{
		&webPageDownloadTestSample{
			expectedNotification: defaultExpectedNotification,
			url:                  url,
		},
	}
}

func (f *normalFile) download(res *webPageDownloadTestResource) action.Action {
	dlButton := nodewith.Name("Download").Role(role.Link).First()
	return uiauto.NamedAction("download video file", res.ui.LeftClick(dlButton))
}

func (f *normalFile) verify(res *webPageDownloadTestResource) action.Action {
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

type maliciousFile struct {
	*webPageDownloadTestSample
}

func newMaliciousFile(url, expectedNotification string) *maliciousFile {
	return &maliciousFile{
		&webPageDownloadTestSample{
			expectedNotification: expectedNotification,
			url:                  url,
		},
	}
}

func (mf *maliciousFile) download(res *webPageDownloadTestResource) action.Action {
	dlButton := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.ListItem).Ancestor(nodewith.Role(role.List)).Nth(2)).Linked()
	return uiauto.NamedAction("click malicious link", res.ui.LeftClick(dlButton))
}

func (mf *maliciousFile) verify(res *webPageDownloadTestResource) action.Action {
	notiString := nodewith.Name("Dangerous download blocked")
	return uiauto.NamedAction("wait for dangerous download notification", res.ui.WaitUntilExists(notiString))
}

type suspiciousFile struct {
	*webPageDownloadTestSample
}

func newSuspiciousFile(url, expectedNotification string) *suspiciousFile {
	return &suspiciousFile{
		&webPageDownloadTestSample{
			expectedNotification: expectedNotification,
			url:                  url,
		},
	}
}

func (sf *suspiciousFile) download(res *webPageDownloadTestResource) action.Action {
	return uiauto.Combine("download chrome extension",
		res.ui.LeftClick(nodewith.Name("START FOR FREE").Role(role.Button)),
		res.ui.LeftClick(nodewith.Name("URL from Chrome WebStore").Role(role.TextField)),
		res.kb.TypeAction(warningLink),
		res.ui.LeftClick(nodewith.Name("DOWNLOAD").Role(role.InlineTextBox)),
		res.ui.LeftClick(nodewith.Name("GET .CRX").Role(role.InlineTextBox)),
	)
}

func (sf *suspiciousFile) verify(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		_, err := ash.WaitForNotification(ctx, res.tconn, notificationTimeout, ash.WaitTitle(sf.expectedNotification))
		return err
	}
}

type imageFile struct {
	*webPageDownloadTestSample
}

func newImageFile(url string) *imageFile {
	return &imageFile{
		&webPageDownloadTestSample{
			expectedNotification: defaultExpectedNotification,
			url:                  url,
		},
	}
}

func (imf *imageFile) download(res *webPageDownloadTestResource) action.Action {
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

func (imf *imageFile) verify(res *webPageDownloadTestResource) action.Action {
	notiString := nodewith.HasClass("LargeImageView")
	return uiauto.NamedAction("quick view on image file from notification", res.ui.WaitUntilExists(notiString))
}

type audioVideoFile struct {
	*webPageDownloadTestSample
}

func newAudioVideoFile(url string) *audioVideoFile {
	return &audioVideoFile{
		&webPageDownloadTestSample{
			expectedNotification: defaultExpectedNotification,
			url:                  url,
		},
	}
}

func (avf *audioVideoFile) download(res *webPageDownloadTestResource) action.Action {
	dlButton := nodewith.Name("Download").Role(role.Link).First()
	return uiauto.NamedAction("download audio file", res.ui.LeftClick(dlButton))
}

func (avf *audioVideoFile) verify(res *webPageDownloadTestResource) action.Action {
	return func(ctx context.Context) error {
		// Video file requires more time to finish download.
		timeout := 5 * time.Minute
		if _, err := ash.WaitForNotification(ctx, res.tconn, timeout, ash.WaitTitle(avf.expectedNotification)); err != nil {
			return errors.Wrapf(err, "failed to wait for download completed notification of audio file within %v", notificationTimeout)
		}
		return nil
	}
}
