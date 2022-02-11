// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vlc contains local Tast tests that exercise vlc.
package vlc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// AppName is the name of ARC app.
	AppName     = "VLC"
	version     = "3.4.2"
	packageName = "org.videolan.vlc"
	idPrefix    = "org.videolan.vlc:id/"

	titleID  = idPrefix + "title"
	navDirID = idPrefix + "nav_directories"

	shortTimeout = 15 * time.Second
	longTimeout  = 2 * time.Minute
)

// Vlc holds resources of ARC app VLC player.
type Vlc struct {
	app *apputil.App
}

// NewVLCPlayer returns VLC instance.
func NewVLCPlayer(ctx context.Context, cr *chrome.Chrome, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC) (*Vlc, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, AppName, packageName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create arc resource")
	}
	vlcPlayer := &Vlc{app}
	if err := vlcPlayer.Install(ctx, cr); err != nil {
		return nil, errors.Wrap(err, "failed to install VLC app")
	}
	return vlcPlayer, nil
}

// Install installs Vlc app through Apk downloaded from "https://get.videolan.org/vlc-android",
// because the version installed from the play store will be inconsistent under different accounts.
func (vlc *Vlc) Install(ctx context.Context, cr *chrome.Chrome) error {
	isInstalled, err := vlc.app.A.PackageInstalled(ctx, vlc.app.PkgName)
	if err != nil {
		return errors.Wrap(err, "failed to find if package is installed")
	}
	if isInstalled {
		return nil
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	conn, err := cr.NewConn(ctx, fmt.Sprintf("https://get.videolan.org/vlc-android/%s/", version))
	if err != nil {
		return errors.Wrap(err, "failed to open download page")
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for page load")
	}

	apkName, err := vlc.getApkName(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get apk name")
	}
	testing.ContextLog(ctx, "Start to install ", apkName)

	done := false
	script := fmt.Sprintf(`() => {
		const apkName = '%s';
		const name = "a[href*='" + apkName + "']";
		const els = document.querySelectorAll(name);
		if (els.length <= 0) return false;
		els[0].click();
		return true;
	}`, apkName)
	if err := conn.Call(ctx, &done, script); err != nil {
		return errors.Wrap(err, "failed to execute JavaScript query to click HTML link to download")
	}
	if !done {
		return errors.New("failed to find element to click")
	}

	chromeui := uiauto.New(vlc.app.Tconn)
	dialog := nodewith.NameContaining(fmt.Sprintf("keep %s anyway?", apkName)).Role(role.AlertDialog)
	keepBtn := nodewith.Role(role.Button).Name("KEEP").Ancestor(dialog)
	showInFolder := nodewith.Role(role.Button).Name("SHOW IN FOLDER").HasClass("MdTextButton")
	apkPath := filepath.Join(filesapp.DownloadPath, apkName)
	if err := uiauto.Combine("download and keep apk",
		chromeui.WithTimeout(3*time.Minute).WaitUntilExists(keepBtn),
		chromeui.LeftClick(keepBtn),
		chromeui.WaitUntilExists(showInFolder),
	)(ctx); err != nil {
		return err
	}
	defer os.Remove(apkPath)

	return vlc.app.A.Install(ctx, apkPath, adb.InstallOptionGrantPermissions)
}

// getApkName gets the name of the APK file to install on the DUT.
func (vlc *Vlc) getApkName(ctx context.Context) (string, error) {
	out, err := vlc.app.A.Command(ctx, "getprop", "ro.product.cpu.abi").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get abi: %s", string(out))
	}
	arch := "x86"
	if strings.HasPrefix(string(out), "arm64-v8a") {
		arch = "arm64-v8a"
	}
	return fmt.Sprintf("VLC-Android-%s-%s.apk", version, arch), nil
}

// Launch launches ARC app VLC.
func (vlc *Vlc) Launch(ctx context.Context) error {
	return vlc.app.Launch(ctx)
}

// Close closes ARC app VLC player and cleanup resources.
func (vlc *Vlc) Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error {
	return vlc.app.Close(ctx, cr, hasError, outDir)
}

// EnterAudioFolder enters into audio folder.
func (vlc *Vlc) EnterAudioFolder(ctx context.Context) error {
	return nil // To be implemented.
}

// PlayAudio plays audio.
func (vlc *Vlc) PlayAudio(ctx context.Context, filetype string) error {
	return nil // To be implemented.
}
