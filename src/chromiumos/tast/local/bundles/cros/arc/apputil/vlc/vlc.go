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

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type mediaType int

const (
	videoType mediaType = iota
	audioType
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

	shortTimeout = 5 * time.Second
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
// If vlc app has been installed, it will check the version.
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
		if ver != version {
			return errors.Errorf("version %s has been installed, expected version is %s", ver, version)
		}
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

	uia := uiauto.New(vlc.app.Tconn)
	dialog := nodewith.NameContaining(fmt.Sprintf("keep %s anyway?", apkName)).Role(role.AlertDialog)
	keepBtn := nodewith.Role(role.Button).Name("KEEP").Ancestor(dialog)
	if err := uia.WithTimeout(3 * time.Minute).LeftClick(keepBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to find keep option")
	}
	defer os.Remove(filepath.Join(filesapp.DownloadPath, apkName))

	return uiauto.Combine("install from tote",
		uia.LeftClick(holdingspace.FindTray()),
		uia.DoubleClick(holdingspace.FindDownloadChip().NameContaining(apkName).First()),
		apputil.FindAndClick(vlc.app.D.Object(ui.Text("CONTINUE")), shortTimeout),
		apputil.FindAndClick(vlc.app.D.Object(ui.Text("INSTALL")), shortTimeout),
		apputil.FindAndClick(vlc.app.D.Object(ui.Text("DONE")), longTimeout),
	)(ctx)
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
	if _, err := vlc.app.Launch(ctx); err != nil {
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
		return errors.Wrapf(err, "[ERR-7002] Can't find the target %s audio file", filetype)
	}

	if err := vlc.clearPromptAfterPlay(ctx, audioType); err != nil {
		return errors.Wrap(err, "failed to clear prompt after play")
	}

	testing.ContextLog(ctx, "Verify playing filename")
	playingFilename := vlc.app.D.Object(ui.ID(titleID), ui.TextContains(filetype))
	if err := playingFilename.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrapf(err, "[ERR-7016] The music(%s) in VLC app is not playing", filetype)
	}

	testing.ContextLog(ctx, "Wait for pause button")
	playPauseID := idPrefix + "header_play_pause"
	pauseButton := vlc.app.D.Object(ui.ID(playPauseID), ui.Description("Pause"))
	if err := pauseButton.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrapf(err, "[ERR-7016] The music(%s) in VLC app is not playing", filetype)
	}

	return nil
}

func (vlc *Vlc) clearStartupPrompt(ctx context.Context) error {
	// If app messages appear, click it.
	testing.ContextLog(ctx, "Clear start up prompt")
	startBtn := vlc.app.D.Object(ui.ID(idPrefix + "startButton"))
	permissionBtn := vlc.app.D.Object(ui.ID(idPrefix + "grantPermissionButton"))

	return uiauto.New(vlc.app.Tconn).IfSuccessThen(
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

func (vlc *Vlc) clearPromptAfterPlay(ctx context.Context, fileType mediaType) error {
	testing.ContextLog(ctx, "Clear instruction prompt")
	nextButton := vlc.app.D.Object(ui.ID(nextBtnID))

	// The multi-step prompt has the same button object. Use for loop to reduce code.
	btnNum := map[mediaType]int{
		audioType: 3,
		videoType: 6,
	}
	for i := 0; i < btnNum[fileType]; i++ {
		if err := apputil.ClickIfExist(nextButton, shortTimeout)(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Pause pauses audio.
func (vlc *Vlc) Pause(ctx context.Context) error {
	return vlc.app.D.PressKeyCode(ctx, ui.KEYCODE_MEDIA_PAUSE, 0)
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

// EnterVideoFolder enters into video folder.
func (vlc *Vlc) EnterVideoFolder(ctx context.Context) error {
	testing.ContextLog(ctx, "Navigate to vlc video folder")
	return uiauto.Combine("navigate to vlc video folder",
		apputil.FindAndClick(vlc.app.D.Object(ui.ID(navDirID)), shortTimeout),
		apputil.FindAndClick(vlc.app.D.Object(ui.ID(titleID), ui.Text("Download")), shortTimeout),
		apputil.FindAndClick(vlc.app.D.Object(ui.ID(titleID), ui.Text("videos")), shortTimeout),
	)(ctx)
}

// PlayVideo plays video.
func (vlc *Vlc) PlayVideo(ctx context.Context, name string) error {
	testing.ContextLogf(ctx, "Playing media file(%s)", name)

	testing.ContextLog(ctx, "Click on file")
	filename := vlc.app.D.Object(ui.Text(name))
	if err := apputil.FindAndClick(filename, shortTimeout)(ctx); err != nil {
		return errors.Wrapf(err, "[ERR-7002] Can't find the target video file: %s", name)
	}

	if err := vlc.clearPromptAfterPlay(ctx, videoType); err != nil {
		return errors.Wrap(err, "failed to clear prompt after play")
	}

	return vlc.VideoIsPlaying(ctx)
}

// VideoIsPlaying ensures the video is playing.
func (vlc *Vlc) VideoIsPlaying(ctx context.Context) error {
	testing.ContextLog(ctx, "Verify if the video is playing")
	prevTime := ""
	curTime := ""
	playBtn := vlc.app.D.Object(ui.ID(idPrefix+"player_overlay_play"), ui.Description("Play"))

	for _, playtime := range []struct {
		name   string
		timing *string
	}{
		{"prev", &prevTime},
		{"cur", &curTime},
	} {
		testing.ContextLogf(ctx, "Get %sTime", playtime.name)
		if err := testing.Poll(ctx, func(c context.Context) error {
			// Press 'Space' to pause and show the progress bar.
			if err := vlc.app.D.PressKeyCode(ctx, ui.KEYCODE_SPACE, 0); err != nil {
				return err
			}
			t, err := vlc.app.D.Object(ui.ID(idPrefix + "player_overlay_time")).GetText(ctx)
			if err != nil || t == "" {
				if clickerr := apputil.ClickIfExist(playBtn, 2*time.Second)(ctx); clickerr != nil {
					return clickerr
				}
				return errors.Wrap(err, "[ERR-3326] Getting currentTime from media element failed")
			}

			*playtime.timing = t
			if playtime.name == "cur" && curTime == prevTime {
				return errors.Wrapf(err, "video isn't playing (previous time: %s, current time: %s) ", prevTime, curTime)
			}
			return apputil.ClickIfExist(playBtn, 2*time.Second)(ctx)
		}, &testing.PollOptions{Timeout: longTimeout, Interval: 2 * time.Second}); err != nil {
			return err
		}
	}
	testing.ContextLogf(ctx, "Prev time: %s, Current time: %s", prevTime, curTime)
	return nil
}
