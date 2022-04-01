// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testutil

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
)

// RobloxTestParams stores data common to all Roblox tests.
type RobloxTestParams struct {
	TestParams
	Uda *uidetection.Context
	Kbd *input.KeyboardEventWriter
}

const (
	// RobloxPkgName is the Android App package name for Roblox.
	RobloxPkgName = "com.roblox.client"
	// RobloxActivity is the Android activity used to start Roblox.
	RobloxActivity = ".startup.ActivitySplash"
	// The inputs rendered by Roblox are not immediately active after being clicked
	// so wait a moment for the engine to make the input active before interacting with it.
	waitForActiveInputTime = time.Second * 5
)

// RobloxLogin installs Roblox from the play store, and logs in with the provided credentials.
func RobloxLogin(ctx context.Context, params TestParams, username, password string) (*RobloxTestParams, error) {
	// onAppReady: Landing will appear in logcat after the game is fully loaded.
	if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`))); err != nil {
		return nil, errors.Wrap(err, "onAppReady was not found in LogCat")
	}

	kbd, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create keyboard")
	}
	uda := uidetection.NewDefault(params.TestConn).WithOptions(uidetection.Retries(3)).WithTimeout(time.Minute)
	if err := uiauto.Combine("Roblox Login",
		// Click the button to start the log in process.
		uda.Tap(uidetection.TextBlock([]string{"Log", "In"})),

		// Click the Username/Email/Phone field and type the username.
		uda.Tap(uidetection.Word("Username/Email/Phone")),
		action.Sleep(waitForActiveInputTime),
		kbd.TypeAction(username),

		// Click the password field and type the password.
		uda.Tap(uidetection.Word("Password").First()),
		action.Sleep(waitForActiveInputTime),
		kbd.TypeAction(password),

		// Click the log in button.
		uda.Tap(uidetection.TextBlock(strings.Split("Log In", " ")).First()),
	)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to login to Roblox")
	}
	return &RobloxTestParams{TestParams: params, Uda: uda, Kbd: kbd}, nil
}

// RobloxMinigameData returns the data files needed by a Roblox test
func RobloxMinigameData(icon string, extraData ...string) []string {
	return append([]string{"roblox-home-screen-search-input.png", "roblox-launch-game.png", icon}, extraData...)
}

// RobloxMinigame navigates from the Roblox start screen to a minigame.
// dataPath - The test's s.DataPath.
// name - The search term used to find the minigame.
// icon - The icon to look for in the search results.
func RobloxMinigame(ctx context.Context, params *RobloxTestParams, dataPath func(string) string, name, icon string) error {
	uda := params.Uda
	kbd := params.Kbd
	if err := uiauto.Combine("Roblox launch minigame",
		// A 'verify your account' prompt occasionally shows up. Wait for that and click through if necessary.
		action.IfSuccessThen(
			uda.WithTimeout(time.Second*30).WaitUntilExists(uidetection.TextBlock([]string{"Verify"})),
			uda.Tap(uidetection.TextBlock([]string{"Verify"})),
		),
		// Click the search dialog, type the game name, and hit 'ENTER' to send the query.
		uda.Tap(uidetection.CustomIcon(dataPath("roblox-home-screen-search-input.png"))),
		action.Sleep(waitForActiveInputTime),
		kbd.TypeAction(name),
		kbd.TypeKeyAction(input.KEY_ENTER),

		// Click the game icon to open the modal.
		uda.Tap(uidetection.CustomIcon(dataPath(icon))),

		// Click the 'launch' button in the game modal.
		uda.Tap(uidetection.CustomIcon(dataPath("roblox-launch-game.png"))),
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to launch Roblox minigame %q", name)
	}
	return nil
}
