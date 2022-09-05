// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
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
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testConfig{
				chromeArgs: []string{
					"--disable-features=ArcEnableVirtioBlkForData,GuestOsFiles",
				},
			},
		}, {
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
	testMountAndroidFiles(ctx, s, tconn, a, "/media/fuse/android_files")
}

// testMountAndroidFiles checks whether a file put in the Android Play files directory
// appears in the host through sshfs.
func testMountAndroidFiles(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, myFilesPath string) {
	const (
		filename    = "capybara.jpg"
		androidPath = "/storage/emulated/0/Pictures/" + filename
	)
	crosPath := myFilesPath + "/" + filename

	testing.ContextLog(ctx, "Testing Android -> CrOS")

	if err := testPushToARCAndMountAndroidFiles(ctx, s, tconn, a, s.DataPath(filename), androidPath, crosPath); err != nil {
		s.Fatal("Android -> CrOS failed: ", err)
	}
}

// testPushToARCAndMountAndroidFiles pushes the content of sourcePath (in ChromeOS)
// to androidPath (in Android) using adb, and then checks whether the file can
// be accessed under crosPath (in ChromeOS).
func testPushToARCAndMountAndroidFiles(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, sourcePath, androidPath, crosPath string) (retErr error) {
	// Shorten the context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	expected, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read from %s in ChromeOS", sourcePath)
	}

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
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	if err := filesApp.OpenPlayfiles()(ctx); err != nil {
		s.Fatal("Failed to open Play files: ", err)
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
