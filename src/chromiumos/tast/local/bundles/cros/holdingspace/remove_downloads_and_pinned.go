// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

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
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type removeTestResource struct {
	cr    *chrome.Chrome
	files *filesapp.FilesApp
	tconn *chrome.TestConn
	ui    *uiauto.Context
	kb    *input.KeyboardEventWriter
	tote  *holdingspace.HoldingSpace
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveDownloadsAndPinned,
		Desc:         "Check if the download/pinned file can be removed from holdingspace after it is deleted",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      5 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

// RemoveDownloadsAndPinned checks if the download/pinned file can be removed from holdingspace after it is deleted.
func RemoveDownloadsAndPinned(ctx context.Context, s *testing.State) {
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

	res := &removeTestResource{
		cr:    cr,
		files: files,
		tconn: tconn,
		ui:    uiauto.New(tconn),
		kb:    kb,
		tote:  holdingspace.New(tconn),
	}

	sampleFile := &sampleFile{
		removeTestResource: res,
		url:                "https://samplelib.com/sample-jpeg.html",
		name:               "sample-clouds-400x300.jpg",
	}

	for _, test := range []struct {
		description string
		instance    removeItem
	}{
		{"downloaded file", &removeDownloaded{res, sampleFile}},
		{"downloaded and pinned file", &removePinned{res, sampleFile}},
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

type sampleFile struct {
	*removeTestResource
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

type removeItem interface {
	prepare(ctx context.Context) error
	remove(ctx context.Context) error
	verify(ctx context.Context) error
	cleanup(ctx context.Context) error
}

type removeDownloaded struct {
	*removeTestResource
	sample *sampleFile
}

func (r *removeDownloaded) prepare(ctx context.Context) error {
	return r.sample.download(ctx)
}

func (r *removeDownloaded) remove(ctx context.Context) error {
	img := holdingspace.FindDownloadChip().Name(r.sample.name)
	return uiauto.Combine("open tote and remove file",
		r.tote.Expand(),
		r.ui.WaitUntilExists(img),
		r.tote.RemoveItem(img),
	)(ctx)
}

func (r *removeDownloaded) verify(ctx context.Context) error {
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

func (r *removeDownloaded) cleanup(ctx context.Context) error {
	return r.sample.cleanup(ctx)
}

type removePinned struct {
	*removeTestResource
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
