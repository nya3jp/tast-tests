// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	"chromiumos/tast/local/chrome/uiauto/wmp"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type fileInToteTestResource struct {
	cr    *chrome.Chrome
	files *filesapp.FilesApp
	tconn *chrome.TestConn
	ui    *uiauto.Context
	kb    *input.KeyboardEventWriter
	tote  *holdingspace.HoldingSpace
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveItemsFromTote,
		Desc:         "Check if file is removed from tote after it is deleted",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      5 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

// RemoveItemsFromTote checks if file is removed from tote after it is deleted.
func RemoveItemsFromTote(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files: ", err)
	}
	defer files.Close(cleanupCtx)

	res := &fileInToteTestResource{
		cr:    cr,
		files: files,
		tconn: tconn,
		ui:    uiauto.New(tconn),
		kb:    kb,
		tote:  holdingspace.New(tconn),
	}

	sampleFile := &sampleFile{
		fileInToteTestResource: res,
		url:                    "https://samplelib.com/sample-jpeg.html",
		name:                   "sample-clouds-400x300.jpg",
	}

	for _, test := range []struct {
		description string
		instance    removeItem
	}{
		{"downloaded file from Tote", &removeFromTote{res, sampleFile}},
		{"downloaded file from Files app", &removeFromFiles{res, sampleFile}},
		{"screenshot from Files app", &removeScreenshot{res}},
		{"downloaded, pinned file from Files app", &removePinned{res, sampleFile}},
	} {
		f := func(ctx context.Context, s *testing.State) {
			subTestCleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			uiName := strings.ReplaceAll(test.description, " ", "_")

			err := test.instance.prepare(ctx)
			defer test.instance.cleanup(subTestCleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, uiName)

			if err != nil {
				s.Fatalf("Failed to prepare for test %q: %v", test.description, err)
			}

			if err := test.instance.remove(ctx); err != nil {
				s.Fatalf("Failed to remove for test %q: %v", test.description, err)
			}

			if err := test.instance.verify(ctx); err != nil {
				s.Fatalf("Failed to verify for test %q: %v", test.description, err)
			}
		}

		if !s.Run(ctx, test.description, f) {
			s.Errorf("Failed to test remove a %s", test.description)
		}
	}
}

type removeItem interface {
	prepare(ctx context.Context) error
	remove(ctx context.Context) error
	verify(ctx context.Context) error
	cleanup(ctx context.Context) error
}

type removeFromTote struct {
	*fileInToteTestResource
	sample *sampleFile
}

func (r *removeFromTote) prepare(ctx context.Context) error {
	return r.sample.download(ctx)
}

func (r *removeFromTote) remove(ctx context.Context) error {
	img := holdingspace.FindDownloadChip().Name(r.sample.name)
	return uiauto.Combine("open tote and remove file",
		r.tote.Expand(),
		r.ui.WaitUntilExists(img),
		r.tote.RemoveItem(img),
	)(ctx)
}

func (r *removeFromTote) verify(ctx context.Context) error {
	toteItem := holdingspace.FindDownloadChip().Name(r.sample.name)
	if err := r.ui.WithTimeout(5 * time.Second).WaitUntilGone(holdingspace.FindTray())(ctx); err != nil {
		return uiauto.Combine("open tote and verify file is removed from tote",
			r.tote.Expand(),
			r.ui.WaitUntilGone(toteItem),
			r.tote.Collapse(),
		)(ctx)
	}
	return nil
}

func (r *removeFromTote) cleanup(ctx context.Context) error {
	return r.sample.cleanup(ctx)
}

type removeFromFiles struct {
	*fileInToteTestResource
	sample *sampleFile
}

func (r *removeFromFiles) prepare(ctx context.Context) error {
	return r.sample.download(ctx)
}

func (r *removeFromFiles) remove(ctx context.Context) error {
	toteItem := holdingspace.FindDownloadChip().Name(r.sample.name)
	return uiauto.Combine("open tote and check if file is in tote, then delete file from Files app",
		r.tote.Expand(),
		r.ui.WaitUntilExists(toteItem),
		r.tote.Collapse(),
		r.files.OpenDownloads(),
		r.files.DeleteFileOrFolder(r.kb, r.sample.name),
		r.files.WaitUntilFileGone(r.sample.name),
	)(ctx)
}

func (r *removeFromFiles) verify(ctx context.Context) error {
	img := holdingspace.FindDownloadChip().Name(r.sample.name)
	if err := r.ui.WithTimeout(5 * time.Second).WaitUntilGone(holdingspace.FindTray())(ctx); err != nil {
		return uiauto.Combine("open tote and verify file is removed from tote",
			r.tote.Expand(),
			r.ui.WaitUntilGone(img),
			r.tote.Collapse(),
		)(ctx)
	}
	return nil
}

func (r *removeFromFiles) cleanup(ctx context.Context) error {
	return r.sample.cleanup(ctx)
}

type removePinned struct {
	*fileInToteTestResource
	sample *sampleFile
}

func (r *removePinned) prepare(ctx context.Context) error {
	if err := r.sample.download(ctx); err != nil {
		return err
	}

	img := holdingspace.FindDownloadChip().Name(r.sample.name)
	pinItem := holdingspace.FindPinnedFileChip().Name(r.sample.name)
	return uiauto.Combine("open tote and pin the file",
		r.tote.Expand(),
		r.tote.PinItem(img),
		r.ui.WaitUntilExists(pinItem),
		r.tote.Collapse(),
	)(ctx)
}

func (r *removePinned) remove(ctx context.Context) error {
	return uiauto.Combine("delete file from Files app and check if it is removed from tote",
		r.files.OpenDownloads(),
		r.files.DeleteFileOrFolder(r.kb, r.sample.name),
		r.files.WaitUntilFileGone(r.sample.name),
	)(ctx)
}

func (r *removePinned) verify(ctx context.Context) error {
	if err := r.ui.WaitUntilGone(holdingspace.FindRootFinder())(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for tote to disappear")
	}
	return nil
}

func (r *removePinned) cleanup(ctx context.Context) error {
	return r.sample.cleanup(ctx)
}

type removeScreenshot struct {
	*fileInToteTestResource
}

func (r *removeScreenshot) prepare(ctx context.Context) error {
	if err := removeAllScreenshots(ctx); err != nil {
		return errors.Wrap(err, "failed to remove all screenshots in advance")
	}
	return takeScreenshot(ctx, r.fileInToteTestResource)
}

func (r *removeScreenshot) remove(ctx context.Context) error {
	return uiauto.Combine("open tote and delete screenshot",
		r.tote.Expand(),
		r.ui.WaitUntilExists(holdingspace.FindScreenCaptureView()),
		r.tote.Collapse(),
		removeAllScreenshots,
	)(ctx)
}

func (r *removeScreenshot) verify(ctx context.Context) error {
	if err := r.ui.WithTimeout(5 * time.Second).WaitUntilGone(holdingspace.FindTray())(ctx); err != nil {
		return uiauto.Combine("open tote and verify file is removed from tote",
			r.tote.Expand(),
			r.ui.WaitUntilGone(holdingspace.FindScreenCaptureView()),
			r.tote.Collapse(),
		)(ctx)
	}
	return nil
}

func (r *removeScreenshot) cleanup(ctx context.Context) error {
	if err := wmp.QuitScreenshot(r.ui)(ctx); err != nil {
		return errors.Wrap(err, "failed to quit from screenshot mode")
	}
	return removeAllScreenshots(ctx)
}

type sampleFile struct {
	*fileInToteTestResource
	url  string
	name string
}

// download opens the page and downloads image from the page.
func (s *sampleFile) download(ctx context.Context) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	page, err := s.cr.NewConn(ctx, s.url)
	if err != nil {
		return errors.Wrap(err, "failed to open page")
	}
	defer page.Close()
	defer page.CloseTarget(cleanupCtx)

	if err := webutil.WaitForQuiescence(ctx, page, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for image page to achieve quiescence")
	}

	waitForDownloadComplete := func(ctx context.Context) error {
		if _, err := ash.WaitForNotification(ctx, s.tconn, time.Minute, ash.WaitTitle("Download complete")); err != nil {
			return errors.Wrap(err, "failed to wait for notification")
		}
		return nil
	}

	if err := uiauto.Combine("download the file",
		s.ui.LeftClick(nodewith.Name("Download").Role(role.Link).First()),
		waitForDownloadComplete,
	)(ctx); err != nil {
		return err
	}

	if err := ash.CloseNotifications(ctx, s.tconn); err != nil {
		return errors.Wrap(err, "failed to close notifications")
	}

	return nil
}

func (s *sampleFile) cleanup(ctx context.Context) error {
	files, err := filepath.Glob(filepath.Join(filesapp.DownloadPath, s.name))
	if err != nil {
		testing.ContextLogf(ctx, "The pattern %q is malformed: %q", s.name, err)
		return errors.Wrapf(err, "the pattern %q is malformed", s.name)
	}

	for _, f := range files {
		if err := os.Remove(f); err != nil {
			testing.ContextLogf(ctx, "Failed to remove file %q: %v", f, err)
			return errors.Wrapf(err, "failed to remove file %q", f)
		}
	}

	return nil
}

func takeScreenshot(ctx context.Context, res *fileInToteTestResource) error {
	if err := wmp.CaptureScreenshot(res.tconn, wmp.FullScreen)(ctx); err != nil {
		return errors.Wrap(err, "failed to take screenshot")
	}

	if err := ash.CloseNotifications(ctx, res.tconn); err != nil {
		return errors.Wrap(err, "failed to close notifications")
	}

	return nil
}

func removeAllScreenshots(ctx context.Context) error {
	pattern := "Screenshot*.png"
	files, err := filepath.Glob(filepath.Join(filesapp.DownloadPath, pattern))
	if err != nil {
		testing.ContextLogf(ctx, "The pattern %q is malformed: %q", pattern, err)
		return errors.Wrapf(err, "the pattern %q is malformed", pattern)
	}

	for _, f := range files {
		if err := os.Remove(f); err != nil {
			testing.ContextLogf(ctx, "Failed to delete the screenshot %q: %v", f, err)
			return errors.Wrap(err, "failed to delete the screenshot")
		}
	}

	return nil
}
