// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// arc.Fsp / arc.Fsp.vm tast tests make use of an unarchiver to mount a test zip file
// on a pseudo file system, accessible by FSP. An Android app is then used to read a
// file on this file system via FSP.

const (
	// fspURI contains the FSP id of Wicked Good Unarchiver.
	// It is defined in ui/file_manager/file_manager/background/js/mount_metrics.js
	fspURI = "content://org.chromium.arc.chromecontentprovider/externalfile%3Amljpablpddhocfbnokacjggdbmafjnon%253A~%25252FMyFiles%25252Farc_fsp_storage%25252Ezip%253A"
	// fspZipFile is the name of the test zip file.
	fspZipFile = "arc_fsp_storage.zip"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Fsp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Android app can read files on pseudo file systems using File System Provider (FSP) via FilesApp",
		Contacts: []string{
			"youkichihosoi@chromium.org",
			"arc-storage@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Data:         []string{fspZipFile},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

func Fsp(ctx context.Context, s *testing.State) {
	// GAIA login is required to use Chrome Web Store.
	cr, err := chrome.New(
		ctx,
		chrome.ARCEnabled(),
		chrome.UnRestrictARCCPU(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Ensure the existence of Text app. This is because TestOpenWithAndroidApp expects that
	// there is at least one app (other than ArcFileReaderTest, the Android app installed in
	// TestOpenWithAndroidApp) that can open a text file.
	if err := installTextAppIfNotInstalled(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to ensure the existence of Text app: ", err)
	}

	// Install the unarchiver Chrome app, that supports FSP.
	unarchiverName := "Wicked Good Unarchiver"
	unarchiverURL := "https://chrome.google.com/webstore/detail/wicked-good-unarchiver/mljpablpddhocfbnokacjggdbmafjnon?hl=en"
	app := cws.App{Name: unarchiverName, URL: unarchiverURL}
	if err := cws.InstallApp(ctx, cr.Browser(), tconn, app); err != nil {
		s.Fatal("Chrome app installation failed: ", err)
	}

	userPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get the cryptohome user path for %s: %v", cr.NormalizedUser(), err)
	}

	destPath := filepath.Join(userPath, "MyFiles", fspZipFile)
	if err := fsutil.CopyFile(s.DataPath(fspZipFile), destPath); err != nil {
		s.Fatalf("Failed to copy %s to %s: %v", fspZipFile, destPath, err)
	}

	// By unzipping, it will create a pseudo file system accessible by FSP.
	if err := unzipFile(ctx, tconn, fspZipFile, "My files", unarchiverName); err != nil {
		s.Fatal("Unzip test zip file failed: ", err)
	}

	config := storage.TestConfig{DirName: fspZipFile, DirTitle: "Files - " + fspZipFile,
		FileName: "storage.txt"}
	expect := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Value: constructFSPURI(userPath, config.FileName)},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, d, config, expect)
}

func installTextAppIfNotInstalled(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	const (
		textAppName = "Text"
		textAppURL  = "https://chrome.google.com/webstore/detail/text/mmfbcljfglbokpmkimbfghdkjmjhdgbg"
		textAppID   = "mmfbcljfglbokpmkimbfghdkjmjhdgbg"
	)

	installed, err := ash.ChromeAppInstalled(ctx, tconn, textAppID)
	if err != nil {
		return errors.Wrap(err, "failed to check the existence of Text app")
	}
	if installed {
		return nil
	}
	testing.ContextLog(ctx, "Installing the missing Text app")
	textApp := cws.App{Name: textAppName, URL: textAppURL}
	return cws.InstallApp(ctx, cr.Browser(), tconn, textApp)
}

// unzipFile unzips the specified "zipFile" located at "folder" using the "unarchiver".
func unzipFile(ctx context.Context, tconn *chrome.TestConn, zipFile, folder, unarchiver string) error {
	msg := "Opening the test zip file with " + unarchiver
	testing.ContextLog(ctx, msg)

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "launching the Files App failed")
	}
	// Close the Files App window to avoid having two windows in TestOpenWithAndroidApp.
	defer files.Close(ctx)

	return uiauto.Combine(msg,
		files.OpenPath("Files - "+folder, folder),
		files.SelectFile(zipFile),
		files.LeftClick(nodewith.Name("Open").Role(role.Button)),
		files.LeftClick(nodewith.Name(unarchiver).Role(role.StaticText)),
	)(ctx)
}

// constructFSPURI constructs a FSP URI.
func constructFSPURI(path, file string) string {
	hash := strings.ReplaceAll(path, "/home/user/", "")
	return fspURI + hash + url.PathEscape("/") + file
}
