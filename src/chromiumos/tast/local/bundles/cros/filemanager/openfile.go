// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	jpegData = "test_jpeg.jpeg"
	jpgData  = "scan_source.jpg"
	pdfData  = "font-test.pdf"
	txtData  = "data_files_external.txt"
	rawData  = "raw_image.raw"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Openfile,
		Desc:         "Test that basic file types supported by files app",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{jpegData, jpgData, pdfData, txtData, rawData},
		Fixture:      "chromeLoggedIn",
	})
}

// Openfile opens each file and checks if file can be opened.
func Openfile(ctx context.Context, s *testing.State) {
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
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(cleanupCtx)

	for _, v := range []verifier{
		newJpeg(files, tconn, kb, jpegData, s.DataPath(jpegData)),
		newJpg(files, tconn, kb, jpgData, s.DataPath(jpgData)),
		newPdf(files, tconn, kb, pdfData, s.DataPath(pdfData)),
		newTxt(files, tconn, kb, txtData, s.DataPath(txtData)),
		newRaw(files, tconn, kb, rawData, s.DataPath(rawData)),
	} {
		f := func(ctx context.Context, s *testing.State) {
			cleanupSubTestCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			if err := v.prepare(ctx); err != nil {
				s.Fatalf("Failed to prepare for %s: %v", v.getFileName(), err)
			}
			defer os.Remove(v.getFilePath())

			if err := v.open(ctx); err != nil {
				s.Fatalf("Failed to open for %s: %v", v.getFileName(), err)
			}
			defer v.close(cleanupSubTestCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubTestCtx, s.OutDir(), s.HasError, cr, v.getFileName())

			if err := v.verify(ctx); err != nil {
				s.Fatalf("Failed to verify for %s: %v", v.getFileName(), err)
			}
		}

		if !s.Run(ctx, fmt.Sprintf("test of open supported file: %s", v.getFileName()), f) {
			s.Errorf("Failed to complete test of file: %q", v.getFileName())
		}
	}
}

type verifier interface {
	prepare(ctx context.Context) error
	open(ctx context.Context) error
	close(ctx context.Context) error
	verify(ctx context.Context) error

	getFilePath() string
	getFileName() string
}
type fileBase struct {
	*filesapp.FilesApp
	tconn              *chrome.TestConn
	kb                 *input.KeyboardEventWriter
	name, source, path string
}

func newFile(files *filesapp.FilesApp, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, name, source string) *fileBase {
	return &fileBase{name: name, source: source, FilesApp: files, tconn: tconn, kb: kb}
}

func (f *fileBase) prepare(ctx context.Context) error {
	f.path = filepath.Join(filesapp.DownloadPath, f.name)
	return fsutil.CopyFile(f.source, f.path)
}

func (f *fileBase) open(ctx context.Context) error {
	return uiauto.Combine("open file from files app",
		f.OpenDownloads(),
		f.WaitForFile(f.name),
		f.OpenFile(f.name),
	)(ctx)
}

func (f *fileBase) close(ctx context.Context) error {
	return f.kb.AccelAction("Ctrl+W")(ctx)
}

func (f *fileBase) getFilePath() string { return f.path }
func (f *fileBase) getFileName() string { return f.name }

type jpegVerifier struct{ *fileBase }
type jpgVerifier struct{ *fileBase }
type pdfVerifier struct{ *fileBase }
type txtVerifier struct{ *fileBase }
type rawVerifier struct{ *fileBase }

func newJpeg(files *filesapp.FilesApp, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, name, source string) *jpegVerifier {
	return &jpegVerifier{newFile(files, tconn, kb, name, source)}
}
func newJpg(files *filesapp.FilesApp, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, name, source string) *jpgVerifier {
	return &jpgVerifier{newFile(files, tconn, kb, name, source)}
}
func newPdf(files *filesapp.FilesApp, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, name, source string) *pdfVerifier {
	return &pdfVerifier{newFile(files, tconn, kb, name, source)}
}
func newTxt(files *filesapp.FilesApp, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, name, source string) *txtVerifier {
	return &txtVerifier{newFile(files, tconn, kb, name, source)}
}
func newRaw(files *filesapp.FilesApp, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, name, source string) *rawVerifier {
	return &rawVerifier{newFile(files, tconn, kb, name, source)}
}

func (f *jpegVerifier) verify(ctx context.Context) error {
	ui := uiauto.New(f.tconn)
	return uiauto.Combine("verify if jpeg is opened",
		ui.WaitUntilExists(nodewith.Name(jpegData).Role(role.Image)),
		checkImageFiles(ui),
	)(ctx)
}

func (f *jpgVerifier) verify(ctx context.Context) error {
	ui := uiauto.New(f.tconn)
	return uiauto.Combine("verify if jpg is opened",
		ui.WaitUntilExists(nodewith.Name(jpgData).Role(role.Image)),
		checkImageFiles(ui),
	)(ctx)
}

func (f *pdfVerifier) verify(ctx context.Context) error {
	ui := uiauto.New(f.tconn)
	return uiauto.Combine("verify if jpeg is opened",
		ui.WaitUntilExists(nodewith.Name("• Japanese characters with “Hiragino Kaku Gothic” Font (CID) → テスト").Role(role.InlineTextBox)),
		ui.WaitUntilExists(nodewith.Name("Menu").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Page number").Role(role.TextField)),
		ui.WaitUntilExists(nodewith.Name("Zoom out").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Zoom level").Role(role.TextField)),
		ui.WaitUntilExists(nodewith.Name("Zoom in").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Fit to page").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Rotate counterclockwise").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Annotate").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Download").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Print").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("More actions").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Thumbnail for page 1").Role(role.Button)),
	)(ctx)
}

func (f *txtVerifier) verify(ctx context.Context) error {
	ui := uiauto.New(f.tconn)
	return uiauto.Combine("verify if jpeg is opened",
		ui.WaitUntilExists(nodewith.Name("This file is stored in Google Cloud Storage.").Role(role.InlineTextBox)),
		ui.WaitUntilExists(nodewith.Name("data_files_external.txt").Role(role.StaticText).HasClass("Label")),
		ui.WaitUntilExists(nodewith.Name("Close").HasClass("TabCloseButton")),
		ui.WaitUntilExists(nodewith.Name("New Tab").HasClass("NewTabButton")),
		ui.WaitUntilExists(nodewith.Name("Search tabs").HasClass("TabSearchButton")),
	)(ctx)
}

func (f *rawVerifier) verify(ctx context.Context) error {
	ui := uiauto.New(f.tconn)
	return uiauto.Combine("verify if raw is opened",
		ui.WaitUntilExists(nodewith.Name(rawData).Role(role.Image)),
		checkImageFiles(ui),
	)(ctx)
}

func checkImageFiles(ui *uiauto.Context) uiauto.Action {
	return uiauto.Combine("verify if image file is opened",
		ui.WaitUntilExists(nodewith.Name("Info").Role(role.ToggleButton)),
		ui.WaitUntilExists(nodewith.Name("Share").Role(role.Button).HasClass("mdc-icon-button mdc-icon-button--display-flex")),
		ui.WaitUntilExists(nodewith.Name("Delete").Role(role.Button).HasClass("mdc-icon-button mdc-icon-button--display-flex")),
		ui.WaitUntilExists(nodewith.Name("Zoom out").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Zoom in").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Crop & rotate").Role(role.ToggleButton)),
		ui.WaitUntilExists(nodewith.Name("Rescale").Role(role.ToggleButton)),
		ui.WaitUntilExists(nodewith.Name("Lighting filters").Role(role.ToggleButton)),
		ui.WaitUntilExists(nodewith.Name("Annotate").Role(role.ToggleButton)),
		ui.WaitUntilExists(nodewith.Name("Undo").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Redo").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("More options").Role(role.ToggleButton)),
	)
}
