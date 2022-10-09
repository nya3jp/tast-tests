// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

type playFilesConfig struct {
	// Extra Chrome command line options.
	chromeArgs []string
	// Path of the Play files mount point in ChromeOS.
	crosPlayfilesPath string
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
			ExtraSoftwareDeps: []string{"android_p"},
			Val: playFilesConfig{
				chromeArgs:        nil,
				crosPlayfilesPath: "/run/arc/sdcard/write/emulated/0",
			},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: playFilesConfig{
				chromeArgs:        nil,
				crosPlayfilesPath: "/run/arc/sdcard/write/emulated/0",
			},
		}, {
			// TODO(b/248151439): Merge to "vm" once the SSHFS version of Play files is enabled on all ARCVM devices.
			Name:              "vm_virtioblk",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: playFilesConfig{
				chromeArgs: []string{
					"--enable-features=ArcEnableVirtioBlkForData,GuestOsFiles",
				},
				crosPlayfilesPath: "/media/fuse/android_files",
			},
		}},
		Timeout: 6 * time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func PlayFiles(ctx context.Context, s *testing.State) {
	config := s.Param().(playFilesConfig)

	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	args := arc.DisableSyncFlags()
	if config.chromeArgs != nil {
		args = append(args, config.chromeArgs...)
	}
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(args...),
	)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Optin is needed to enable the Play files feature.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := arc.WaitForARCSDCardVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for the sdcard volume to be mounted in ARC: ", err)
	}

	testing.ContextLog(ctx, "Testing CrOS -> Android")
	if err := testCrosToAndroid(ctx, s, cr, tconn, a); err != nil {
		s.Fatal("CrOS -> Android failed: ", err)
	}

	testing.ContextLog(ctx, "Testing Android -> CrOS")
	if err := testAndroidToCros(ctx, a, s.DataPath("capybara.jpg"), config.crosPlayfilesPath); err != nil {
		s.Fatal("Android -> CrOS failed: ", err)
	}
}

// testCrosToAndroid checks whether 1) the contents of Play files can be
// manipulated through the Files app, and 2) the results of the manipulations
// are properly reflected on the Android side.
func testCrosToAndroid(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC) error {
	const filename = "storage.txt"

	if err := testCopyToPlayfiles(ctx, cr, tconn, a, filename); err != nil {
		return errors.Wrapf(err, "failed to copy %s to Play files", filename)
	}

	if err := testOpenInPlayfiles(ctx, s, cr, a, filename); err != nil {
		return errors.Wrapf(err, "failed to open %s in Play files", filename)
	}

	if err := testDeleteFromPlayfiles(ctx, tconn, a, filename); err != nil {
		return errors.Wrapf(err, "failed to delete %s from Play files", filename)
	}

	return nil
}

// testCopyToPlayfiles writes a file to the ChromeOS Downloads directory and
// copies it to Play files through the Files app. It also checks that the copied
// file appears on the Android side.
func testCopyToPlayfiles(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, filename string) error {
	expected := []byte(storage.ExpectedFileContent)
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		return errors.Wrap(err, "failed to get user's Download path")
	}
	crosPath := filepath.Join(downloadsPath, filename)
	if err := ioutil.WriteFile(crosPath, expected, 0666); err != nil {
		return errors.Wrapf(err, "failed to write to %s in ChromeOS", crosPath)
	}
	defer os.Remove(crosPath)

	if err := copyFileInDownloadsToPlayfiles(ctx, tconn, filename); err != nil {
		return errors.Wrapf(err, "failed to copy %s through the Files app", filename)
	}

	// Check that the file appears on the Android side.
	androidPath := filepath.Join("/storage/emulated/0/Pictures", filename)
	return testing.Poll(ctx, func(ctx context.Context) error {
		actual, err := a.ReadFile(ctx, androidPath)
		if err != nil {
			return errors.Wrapf(err, "failed to read %s in Android", androidPath)
		}
		if !bytes.Equal(actual, expected) {
			return errors.Errorf("content mismatch between %s in Android and %s in ChromeOS", androidPath, crosPath)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// copyFileInDownloadsToPlayfiles copies a file in Downloads to Play files.
func copyFileInDownloadsToPlayfiles(ctx context.Context, tconn *chrome.TestConn, filename string) error {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch the Files app")
	}
	defer filesApp.Close(cleanupCtx)

	steps := []uiauto.Action{
		// Open "Downloads".
		filesApp.OpenDownloads(),

		// Wait for file to be displayed.
		filesApp.WaitForFile(filename),

		// Copy file.
		filesApp.ClickContextMenuItem(filename, filesapp.Copy),

		// Open "Play files".
		filesApp.OpenPlayfiles(),

		// Paste to "Pictures" directory.
		filesApp.ClickContextMenuItem("Pictures", "Paste into folder"),

		// Open "Pictures".
		filesApp.OpenFile("Pictures"),

		// Wait until file exists.
		filesApp.WaitForFile(filename)}

	return uiauto.Combine("copy file from Downloads to Play files", steps...)(ctx)
}

// testOpenInPlayfiles opens a file in Play files with an Android app.
func testOpenInPlayfiles(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, filename string) error {
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize UI Automator")
	}

	testFileURI := arc.VolumeProviderContentURIPrefix + filepath.Join("external_files", "Pictures", filename)
	config := storage.TestConfig{DirName: "Play files", DirTitle: "Files - Play files", SubDirectories: []string{"Pictures"}, FileName: filename, CreateTestFile: false}
	expectations := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Value: testFileURI},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}
	storage.TestOpenWithAndroidApp(ctx, s, a, cr, d, config, expectations)

	return nil
}

// testDeleteFromPlayfiles deletes a file in Play files through the Files app.
// It also checks that the file is properly deleted on the Android side.
func testDeleteFromPlayfiles(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, filename string) error {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open Files app")
	}
	defer filesApp.Close(cleanupCtx)

	steps := []uiauto.Action{
		// Open "Play files".
		filesApp.OpenPlayfiles(),

		// Open "Pictures".
		filesApp.OpenFile("Pictures"),

		// Wait for file to appear.
		filesApp.WaitForFile(filename),

		// Delete the file.
		filesApp.ClickContextMenuItem(filename, filesapp.Delete),

		// Confirm the deletion.
		filesApp.LeftClick(nodewith.Name("Delete").ClassName("cr-dialog-ok").Role(role.Button)),

		// Wait until file disappear.
		filesApp.WaitUntilFileGone(filename)}

	if err := uiauto.Combine("delete file from Play files", steps...)(ctx); err != nil {
		return err
	}

	// Check that the file is deleted on the Android side.
	androidPath := filepath.Join("/storage/emulated/0/Pictures", filename)
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := a.ReadFile(ctx, androidPath); err == nil {
			return errors.Errorf("%s still exists in Android", androidPath)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// testAndroidToCros creats a file in Android's sdcard volume, and checks that
// the file appears in the corresponding ChromeOS side path.
func testAndroidToCros(ctx context.Context, a *arc.ARC, dataPath, crosPlayfilesPath string) error {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	const androidPath = "/storage/emulated/0/Pictures/capybara.jpg"
	crosPath := filepath.Join(crosPlayfilesPath, "Pictures", "capybara.jpg")

	expected, err := ioutil.ReadFile(dataPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", dataPath)
	}

	if err := a.WriteFile(ctx, androidPath, expected); err != nil {
		return errors.Wrapf(err, "failed to write to %s in Android", androidPath)
	}
	defer a.RemoveAll(cleanupCtx, androidPath)

	actual, err := ioutil.ReadFile(crosPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s in ChromeOS", crosPath)
	}

	if !bytes.Equal(actual, expected) {
		return errors.Wrapf(err, "content mismatch between %s in ChromeOS and %s in Android", crosPath, androidPath)
	}

	return nil
}
