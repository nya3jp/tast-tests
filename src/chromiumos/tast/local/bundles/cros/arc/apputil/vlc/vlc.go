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
	"chromiumos/tast/common/android/ui"
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

	titleID   = idPrefix + "title"
	navDirID  = idPrefix + "nav_directories"
	doneBtnID = idPrefix + "doneButton"
	nextBtnID = idPrefix + "nextButton"

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
// If the wrong version is installed, it will reinstall.
func (vlc *Vlc) Install(ctx context.Context, cr *chrome.Chrome) error {
	isInstalled, err := vlc.app.A.PackageInstalled(ctx, vlc.app.PkgName)
	if err != nil {
		return errors.Wrap(err, "failed to find if package is installed")
	}
	if isInstalled {
		ver, err := vlc.app.GetVersion(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get installed version")
		}
		if ver == version {
			return nil
		}
		testing.ContextLogf(ctx, "Version %s has been installed, reinstall version %s", ver, version)
		if err := vlc.app.A.Uninstall(ctx, vlc.app.PkgName); err != nil {
			return errors.Wrapf(err, "failed to uninstall the wrong version %s", ver)
		}
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
	testing.ContextLogf(ctx, "Openning app: %q", AppName)
	if err := vlc.app.Launch(ctx); err != nil {
		return errors.Wrap(err, "failed to launch App")
	}

	return vlc.clearStartupPrompt(ctx)
}

// Close closes ARC app VLC player and cleanup resources.
func (vlc *Vlc) Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error {
	return vlc.app.Close(ctx, cr, hasError, outDir)
}

// EnterAudioFolder enters into audio folder.
func (vlc *Vlc) EnterAudioFolder(ctx context.Context) error {
	testing.ContextLog(ctx, "Navigate to vlc audio folder")
	return uiauto.Combine("navigate to vlc audio folder",
		apputil.FindAndClick(vlc.app.D.Object(ui.ID(navDirID)), shortTimeout),
		apputil.FindAndClick(vlc.app.D.Object(ui.ID(titleID), ui.Text("Download")), shortTimeout),
		apputil.FindAndClick(vlc.app.D.Object(ui.ID(titleID), ui.Text("audios")), shortTimeout),
	)(ctx)
}

// PlayAudio plays audio.
func (vlc *Vlc) PlayAudio(ctx context.Context, filetype string) error {
	testing.ContextLogf(ctx, "Playing media file(%s)", filetype)

	testing.ContextLog(ctx, "Click on file")
	filename := vlc.app.D.Object(ui.TextContains(filetype))
	if err := apputil.FindAndClick(filename, shortTimeout)(ctx); err != nil {
		return errors.Wrapf(err, "failed to find the target file: %s", filetype)
	}

	if err := vlc.clearPromptAfterPlay(ctx); err != nil {
		return errors.Wrap(err, "failed to clear prompt after play")
	}

	testing.ContextLog(ctx, "Verify playing filename")
	playingFilename := vlc.app.D.Object(ui.ID(titleID), ui.TextContains(filetype))
	if err := playingFilename.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "the VLC player is not playing")
	}

	testing.ContextLog(ctx, "Wait for pause button")
	playPauseID := idPrefix + "header_play_pause"
	pauseButton := vlc.app.D.Object(ui.ID(playPauseID), ui.Description("Pause"))
	if err := pauseButton.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "the VLC player is not playing")
	}

	return nil
}

func (vlc *Vlc) clearStartupPrompt(ctx context.Context) error {
	// If app messages appear, click it.
	testing.ContextLog(ctx, "Clear start up prompt")
	startBtn := vlc.app.D.Object(ui.ID(idPrefix + "startButton"))
	permissionBtn := vlc.app.D.Object(ui.ID(idPrefix + "grantPermissionButton"))

	return uiauto.IfSuccessThen(
		apputil.WaitForExists(startBtn, shortTimeout),
		uiauto.Combine("clear start up prompt",
			apputil.ClickIfExist(startBtn, shortTimeout),
			apputil.ClickIfExist(permissionBtn, shortTimeout),
			apputil.ClickIfExist(vlc.app.D.Object(ui.Text("ALLOW")), shortTimeout),
			apputil.ClickIfExist(vlc.app.D.Object(ui.ID(nextBtnID)), shortTimeout),
			apputil.ClickIfExist(vlc.app.D.Object(ui.ID(doneBtnID)), shortTimeout),
			apputil.ClickIfExist(vlc.app.D.Object(ui.Text("YES")), shortTimeout),
		),
	)(ctx)
}

func (vlc *Vlc) clearPromptAfterPlay(ctx context.Context) error {
	testing.ContextLog(ctx, "Clear instruction prompt")
	nextButton := vlc.app.D.Object(ui.ID(nextBtnID))

	// The multi-step prompt has the same button object. Use for loop to reduce code.
	for i := 0; i < 3; i++ {
		if err := apputil.ClickIfExist(nextButton, shortTimeout)(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Play plays audio.
func (vlc *Vlc) Play(ctx context.Context) error {
	return vlc.app.D.PressKeyCode(ctx, ui.KEYCODE_MEDIA_PLAY, 0)
}

// IsPaused check if the player paused.
func (vlc *Vlc) IsPaused(ctx context.Context) error {
	playPauseID := idPrefix + "header_play_pause"
	playButton := vlc.app.D.Object(ui.ID(playPauseID), ui.Description("Play"))
	if err := playButton.Exists(ctx); err != nil {
		errors.Wrap(err, "play button not found, player is not paused")
	}
	return nil
}

// IsPlaying check if the player is playing.
func (vlc *Vlc) IsPlaying(ctx context.Context) error {
	playPauseID := idPrefix + "header_play_pause"
	playButton := vlc.app.D.Object(ui.ID(playPauseID), ui.Description("Pause"))
	if err := playButton.Exists(ctx); err != nil {
		errors.Wrap(err, "pause button not found, player is not playing")
	}
	return nil
}
