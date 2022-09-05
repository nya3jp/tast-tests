// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

type testConfig struct {
	// Extra Chrome command line options
	chromeArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayFiles,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks whether the Play files directory is properly shared from ARC to ChromeOS",
		Contacts: []string{
			"youkichihosoi@chromium.org", "arc-storage@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"capybara.jpg"},
		Params: []testing.Param{{
			Name:              "vm_virtioblk",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testConfig{
				chromeArgs: []string{
					"--enable-features=ArcEnableVirtioBlkForData,GuestOsFiles",
				},
			},
		}},
		Timeout: 6 * time.Minute,
	})
}

func PlayFiles(ctx context.Context, s *testing.State) {
	const (
		filename    = "capybara.jpg"
		androidPath = "/storage/emulated/0/Pictures/" + filename
		crosPath    = "/media/fuse/android_files/Pictures/" + filename
	)

	s.Log("Log into another Chrome instance")
	args := s.Param().(testConfig).chromeArgs
	cr, err := chrome.New(
		ctx,
		chrome.ARCEnabled(),
		chrome.ExtraArgs(args...),
	)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	if err := arc.WaitForARCSDCardVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for SDCard to be mounted in ARC: ", err)
	}

	testing.ContextLog(ctx, "Testing Android -> CrOS")

	expected, err := ioutil.ReadFile(s.DataPath(filename))
	if err != nil {
		s.Fatalf("Failed to read from %s in ChromeOS: %v", s.DataPath(filename), err)
	}

	if err := testMountPlayfiles(ctx, tconn, a, androidPath, crosPath, expected); err != nil {
		s.Fatal("Android -> Host failed: ", err)
	}

	if err := testCopyToPlayfiles(ctx, tconn, a, cr, androidPath, expected, filename); err != nil {
		s.Fatal("Host -> Android failed: ", err)
	}
}

// testMountPlayfiles pushes the content of sourcePath (in ChromeOS) to
// androidPath (in Android) using adb, mount Play files by clicking the
// Play files icon in FilesApp, and then checks whether the file can be
// accessed under crosPath (in ChromeOS).
func testMountPlayfiles(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, androidPath, crosPath string, expected []byte) (retErr error) {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := a.WriteFile(ctx, androidPath, expected); err != nil {
		return errors.Wrapf(err, "failed to write to %s in Android", androidPath)
	}
	defer func(ctx context.Context) {
		if err := a.RemoveAll(ctx, androidPath); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed remove %s in Android", androidPath)
			} else {
				testing.ContextLogf(ctx, "Failed to remove %s in Android: %v", androidPath, err)
			}
		}
	}(cleanupCtx)

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open Files app")
	}
	defer filesApp.Close(cleanupCtx)

	if err := filesApp.OpenPlayfiles()(ctx); err != nil {
		return errors.Wrap(err, "failed to open Play files")
	}

	actual, err := ioutil.ReadFile(crosPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read from %s in ChromeOS", crosPath)
	}
	if !bytes.Equal(actual, expected) {
		return errors.Errorf("content mismatch between %s in Android and %s in ChromeOS", androidPath, crosPath)
	}

	return nil
}

// testCopyToPlayfiles writes content to downloads directory (in ChromeOS), copy
// it to Play files through FilesApp, and then checks whether the file can be
// accessed under androidPath (in Android).
func testCopyToPlayfiles(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, cr *chrome.Chrome, androidPath string, expected []byte, filename string) (retErr error) {
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		return errors.Wrap(err, "failed to get user's Download path")
	}

	imageDownloadsPath := filepath.Join(downloadsPath, filename)
	if err := ioutil.WriteFile(imageDownloadsPath, expected, 0644); err != nil {
		return errors.Wrapf(err, "failed to write to %s in Downloads", androidPath)
	}
	defer func() {
		if err := os.Remove(imageDownloadsPath); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed remove %s in Downloads", imageDownloadsPath)
			} else {
				testing.ContextLogf(ctx, "Failed to remove %s in Downloads: %v", imageDownloadsPath, err)
			}
		}
	}()

	copyToPlayfiles(ctx, tconn, filename)

	actual, err := a.ReadFile(ctx, androidPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read from %s in Android", androidPath)
	}

	if !bytes.Equal(actual, expected) {
		return errors.Errorf("content mismatch between %s in Android and in ChromeOS", androidPath)
	}

	deleteFileInPlayfiles(ctx, tconn, filename)

	if _, err := a.FileSize(ctx, androidPath); err == nil {
		return errors.Wrapf(err, "failed to delete %s", androidPath)
	}
	return nil
}

// copyToPlayfiles copies test image in Downloads to Play files.
func copyToPlayfiles(ctx context.Context, tconn *chrome.TestConn, filename string) error {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open Files app")
	}
	defer filesApp.Close(cleanupCtx)

	steps := []uiauto.Action{
		// Open "Downloads"
		filesApp.OpenDownloads(),

		// Wait for file to display
		filesApp.WaitForFile(filename),

		// Copy file.
		filesApp.ClickContextMenuItem(filename, filesapp.Copy),

		// Open "Play files".
		filesApp.OpenPlayfiles(),

		// Paste to "Pictures" directory.
		filesApp.ClickContextMenuItem("Pictures", "Paste into folder"),

		// Open "Pictures"
		filesApp.OpenFile("Pictures"),

		// Wait until file exists
		filesApp.WaitForFile(filename)}

	if err := uiauto.Combine("copy files from Downloads to Play files", steps...)(ctx); err != nil {
		return err
	}

	return nil
}

// deleteFileInPlayfiles delete test image in Play files.
func deleteFileInPlayfiles(ctx context.Context, tconn *chrome.TestConn, filename string) error {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open Files app")
	}
	defer filesApp.Close(cleanupCtx)

	steps := []uiauto.Action{
		// Open "Play files"
		filesApp.OpenPlayfiles(),

		// Open "Pictures"
		filesApp.OpenFile("Pictures"),

		// Wait for file to appear
		filesApp.WaitForFile(filename),

		// Delete the file
		filesApp.ClickContextMenuItem(filename, filesapp.Delete),

		// Confirm the deletion
		filesApp.LeftClick(nodewith.Name("Delete").ClassName("cr-dialog-ok").Role(role.Button)),

		// Wait until file disappear
		filesApp.WaitUntilFileGone(filename)}

	if err := uiauto.Combine("delete file from Play files", steps...)(ctx); err != nil {
		return err
	}

	return nil
}
