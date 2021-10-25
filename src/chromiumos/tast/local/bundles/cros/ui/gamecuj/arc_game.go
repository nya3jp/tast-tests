// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gamecuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	gameAppName     = "Sonic 4 episode 2"
	gamePackageName = "com.sega.sonic4episode2"
	gameIDPrefix    = gamePackageName + ":id/"
)

// ArcGame holds the information used to do Game APP testing.
type ArcGame struct {
	kb       *input.KeyboardEventWriter
	tconn    *chrome.TestConn
	a        *arc.ARC
	d        *androidui.Device
	launched bool
}

// NewArcGame creates ArcGame instance which implements GameApp interface.
func NewArcGame(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC, d *androidui.Device) *ArcGame {
	return &ArcGame{
		kb:    kb,
		tconn: tconn,
		a:     a,
		d:     d,
	}
}

var _ GameApp = (*ArcGame)(nil)

// Install installs game app.
func (ag *ArcGame) Install(ctx context.Context) error {
	// Limit the game app installation time with a new context.
	installCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := playstore.InstallOrUpdateApp(installCtx, ag.a, ag.d, gamePackageName, -1); err != nil {
		return errors.Wrapf(err, "failed to install %s", gamePackageName)
	}
	if err := apps.Close(ctx, ag.tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}
	return nil
}

// Launch launches game app.
func (ag *ArcGame) Launch(ctx context.Context) (time.Duration, error) {
	if w, err := ash.GetARCAppWindowInfo(ctx, ag.tconn, gamePackageName); err == nil {
		// If the package is already visible,
		// needs to close it and launch again to collect app start time.
		if err := w.CloseWindow(ctx, ag.tconn); err != nil {
			return -1, errors.Wrapf(err, "failed to close %s app", gameAppName)
		}
	}

	var startTime time.Time
	// Sometimes the Game App will fail to open, so add retry here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := launcher.SearchAndLaunch(ag.tconn, ag.kb, gameAppName)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s app", gameAppName)
		}
		startTime = time.Now()
		return ash.WaitForVisible(ctx, ag.tconn, gamePackageName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return -1, errors.Wrapf(err, "failed to wait for the new window of %s", gamePackageName)
	}

	ag.launched = true
	return time.Since(startTime), nil
}

// End cleans up the Arc resources and closes game app.
func (ag *ArcGame) End(ctx context.Context) error {
	if !ag.launched {
		return nil
	}
	w, err := ash.GetARCAppWindowInfo(ctx, ag.tconn, gamePackageName)
	if err != nil {
		return errors.Wrap(err, "failed to get game window info")
	}
	return w.CloseWindow(ctx, ag.tconn)
}

// Play enters the game scene by keyboard and plays the game by game pad.
func (ag *ArcGame) Play(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {
	if err := ag.enterGameScene(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to enter game scene")
	}

	gp, err := input.Gamepad(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a gamepad")
	}
	defer func() {
		if gp != nil {
			gp.Close()
		}
	}()
	testing.ContextLog(ctx, "Created a virtual gamepad device ", gp.Device())

	const (
		// waitEventTime is the interval time between events to prevent event triggering failure.
		waitEventTime = 100 * time.Millisecond
		// The game character can run back and forth, look up, squat and jump.
		stopX = "stopX" // stopX controls the game character to stop running back and forth.
		stopY = "stopY" // stopX controls the game character to stop look up and squat.
		right = "right" // right controls the game character to run forth.
		left  = "left"  // left controls the game character to run back.
		down  = "down"  // down controls the game character to squat.
		up    = "up"    // up controls the game character to look up.
	)
	// run controls the game charactor to do the expected action.
	run := func(action string, duration time.Duration) action.Action {
		return func(ctx context.Context) error {
			var val int32
			var ec input.EventCode
			switch action {
			case stopX:
				ec = input.ABS_X
				val = gp.Axes()[ec].Flat
			case stopY:
				ec = input.ABS_Y
				val = gp.Axes()[ec].Flat
			case right:
				ec = input.ABS_X
				val = gp.Axes()[ec].Maximum
			case left:
				ec = input.ABS_X
				val = gp.Axes()[ec].Minimum
			case down:
				ec = input.ABS_Y
				val = gp.Axes()[ec].Maximum
			case up:
				ec = input.ABS_Y
				val = gp.Axes()[ec].Minimum
			}
			testing.ContextLog(ctx, "Start to run ", action)
			// wait between events to prevent event triggering failure.
			if err := testing.Sleep(ctx, waitEventTime); err != nil {
				return errors.Wrap(err, "failed to wait before moving axis")
			}
			if err := gp.MoveAxis(ctx, ec, val); err != nil {
				return errors.Wrap(err, "failed to move axis")
			}
			if err := testing.Sleep(ctx, duration); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			return nil
		}
	}
	jumpAndContinue := func(duration time.Duration) action.Action {
		return func(ctx context.Context) error {
			jumpEventCode := input.BTN_EAST
			testing.ContextLog(ctx, "Start to jump")
			// wait between events to prevent event triggering failure.
			if err := testing.Sleep(ctx, waitEventTime); err != nil {
				return errors.Wrap(err, "failed to wait before pressing button")
			}
			if err := gp.PressButton(ctx, jumpEventCode); err != nil {
				return errors.Wrap(err, "failed to press button")
			}
			// wait between events to prevent event triggering failure.
			if err := testing.Sleep(ctx, waitEventTime); err != nil {
				return errors.Wrap(err, "failed to wait before releasing button")
			}
			if err := gp.ReleaseButton(ctx, jumpEventCode); err != nil {
				return errors.Wrap(err, "failed to release button")
			}
			if err := testing.Sleep(ctx, duration); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			return nil
		}
	}

	// In gameplay, follow the steps below to perform actions and
	// let the game progress to a stage.
	testing.ContextLog(ctx, "Start to play the game")
	return uiauto.Combine("play the game",
		run(right, 8*time.Second),
		run(left, 2*time.Second),
		run(right, 4*time.Second),
		run(left, 0),
		jumpAndContinue(0),
		run(right, 6*time.Second),
		run(stopX, 0),
		run(down, 2*time.Second),
		jumpAndContinue(400*time.Millisecond),
		jumpAndContinue(400*time.Millisecond),
		jumpAndContinue(400*time.Millisecond),
		run(stopY, 0),
		run(right, 2*time.Second),
		jumpAndContinue(2*time.Second),
		jumpAndContinue(2*time.Second),
		jumpAndContinue(2*time.Second),
		jumpAndContinue(2*time.Second),
		jumpAndContinue(2*time.Second),
		jumpAndContinue(2*time.Second),
		jumpAndContinue(20*time.Second),
	)(ctx)
}

// enterGameScene initializes user information and enters the game scene.
func (ag *ArcGame) enterGameScene(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// allowAccess allows app to access photo, media and files.
	allowAccess := func(ctx context.Context) error {
		bgImage := ag.d.Object(androidui.ID("com.sega.sonic4episode2:id/bg"))
		okButton := ag.d.Object(androidui.Text("OK"))
		allowButton := ag.d.Object(androidui.Text("ALLOW"))
		if err := cuj.WaitForExists(bgImage, shortUITimeout)(ctx); err != nil {
			testing.ContextLog(ctx, "Start to allow app access photo, media and files")
			return uiauto.Combine("allow app access photo, media and files",
				cuj.FindAndClick(okButton, defaultUITimeout),
				cuj.FindAndClick(allowButton, defaultUITimeout),
				cuj.WaitForExists(bgImage, defaultUITimeout),
			)(ctx)
		}
		return nil
	}

	// maxmizedWindow maximizes the application window.
	maxmizedWindow := func(ctx context.Context) error {
		if state, err := ash.GetARCAppWindowState(ctx, tconn, gamePackageName); err != nil {
			return errors.Wrap(err, "failed to get Ash window state")
		} else if state != ash.WindowStateFullscreen {
			frameCenterButton := nodewith.ClassName("FrameCenterButton").Role(role.Button)
			resizableButton := nodewith.Name("Resizable").Role(role.MenuItem).ClassName("Button")
			allowButton := nodewith.Name("Allow").Role(role.Button)
			testing.ContextLog(ctx, "Select resizable mode")
			if err := uiauto.Combine("select resizable mode",
				ui.LeftClick(frameCenterButton),
				ui.LeftClick(resizableButton),
				ui.LeftClick(allowButton),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to select resizable mode")
			}

			testing.ContextLog(ctx, "Maximize the application window")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, gamePackageName, ash.WMEventMaximize); err != nil {
				return errors.Wrap(err, "failed to maximize the application")
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, gamePackageName, ash.WindowStateMaximized); err != nil {
				return errors.Wrap(err, "failed to wait for application to enter Maximized state")
			}
		}
		return nil
	}
	mainScreen := ag.d.Object(androidui.ID("android:id/title_template"))
	enterMainScreen := func(ctx context.Context) error {
		ageButton := ag.d.Object(androidui.ID(gameIDPrefix + "btn_2"))
		doneButton := ag.d.Object(androidui.Text("DONE"))
		agreeButton := ag.d.Object(androidui.Text("AGREE"))
		if err := uiauto.NamedAction("wait for main screen",
			cuj.WaitForExists(mainScreen, defaultUITimeout))(ctx); err != nil {
			// enterAge enters age in the first time.
			testing.ContextLog(ctx, "Start to enter your age")
			return uiauto.Combine("enter age",
				cuj.FindAndClick(ageButton, defaultUITimeout),
				cuj.FindAndClick(ageButton, defaultUITimeout),
				cuj.FindAndClick(doneButton, defaultUITimeout),
				cuj.FindAndClick(agreeButton, defaultUITimeout),
				cuj.WaitForExists(mainScreen, defaultUITimeout),
			)(ctx)
		}
		return nil
	}
	playButton := ag.d.Object(androidui.ID(gameIDPrefix + "button_play"))
	mainMenu := ag.d.Object(androidui.ID(gameIDPrefix + "btn_removeads"))
	closeADIfExists := func(ctx context.Context) error {
		// Not every account will show the advertisement.
		if err := cuj.WaitForExists(mainMenu, shortUITimeout)(ctx); err == nil {
			return nil
		}
		// Old Chrome OS keyboards send F1 and software maps it to "back".
		// Wilco devices and newer Chrome OS keyboards directly send "back".
		topRow, err := input.KeyboardTopRowLayout(ctx, kb)
		if err != nil {
			return errors.Wrap(err, "failed to obtain keyboard layout")
		}

		back := "F1"
		if topRow.BrowserBack != "F1" {
			back = "back"
		}
		// Press back to close advertisement.
		return uiauto.NamedAction("close advertisement", kb.AccelAction(back))(ctx)
	}
	noButton := ag.d.Object(androidui.Text("No"))
	enterMainMenu := uiauto.Combine("enter main menu",
		uiauto.NamedAction("press J to continue", kb.AccelAction("J")),
		cuj.WaitUntilGone(mainScreen, shortUITimeout),
		closeADIfExists,
		cuj.ClickIfExist(noButton, shortUITimeout),
		// Only the first time need to click "PLAY".
		cuj.ClickIfExist(playButton, shortUITimeout),
		uiauto.NamedAction("waiting for main menu", cuj.WaitForExists(mainMenu, 10*time.Second)),
	)

	okButton := ag.d.Object(androidui.Text("OK"))
	gameAnimation := ag.d.Object(androidui.ClassName("android.view.View"), androidui.Focusable(true), androidui.Index(0))
	gameScene := ag.d.Object(androidui.ClassName("android.view.View"), androidui.Focusable(false), androidui.Index(0))
	defaultOptions := &testing.PollOptions{Interval: 3 * time.Second, Timeout: time.Minute}

	testing.ContextLog(ctx, "Start to enter game scene")
	return uiauto.Combine("enter game scene",
		allowAccess,
		maxmizedWindow,
		enterMainScreen,
		// There is an error message about "Unable to sign into Google Play Services." on some DUTs.
		// If this dialog box appears, just click "OK" button to continue the game.
		cuj.ClickIfExist(okButton, shortUITimeout),
		uiauto.NamedAction("start to enter main menu", ui.Retry(3, enterMainMenu)),
		uiauto.NamedAction("press K to start the game", kb.AccelAction("K")),
		uiauto.NamedAction("waiting to leave the main menu", cuj.WaitUntilGone(mainMenu, defaultUITimeout)),
		uiauto.NamedAction("waiting for game animation", cuj.WaitForExists(gameAnimation, defaultUITimeout)),
		uiauto.NamedAction("waiting for game scene", cuj.DoActionUntilExists(kb.AccelAction("A"), gameScene, defaultOptions)),
	)(ctx)
}
