// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/testing"
	"chromiumos/tast/local/coords"
)

// Hard coded login field heuristics.
const (
  FirstLoginButton    = 0.0
  UsernameField       = 0.0
  UsernameString      = ""
  PasswordField       = 0.0
  PasswordString      = ""
  SecondLoginButton   = 0.0
)

const loginParams := testutil.LoginParams{FirstLoginButton, UsernameField, UsernameString, PasswordField, PasswordString, SecondLoginButton}

func init() {
	testing.AddTest(&testing.Test{
		Func:         RobloxLogin,
		Desc:         "Captures login metrics for Roblox",
		Contacts:     []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				Pre:               pre.ArcAppGamePerfBooted,
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Pre:               pre.ArcAppGamePerfBooted,
			}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

func RobloxLogin(ctx context.Context, s *testing.State) {
  const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
	)

	testutil.PerformLoginTest(ctx, s, appPkgName, appActivity,
	  func(params testutil.TestParams) (isLaunched bool, err error) {
		  // onAppReady: Landing will appear in logcat after the game is fully loaded.
		  if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sLanding`))); err != nil {
		  	return false, errors.Wrap(err, "onAppReady was not found in LogCat")
		  }

		  return true, nil
		},
	  func(params testutil.TestParams) (isLaunched bool, err error) {
		  // onAppReady: Landing will appear in logcat after the game is fully loaded.
		  if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`onAppReady:\sAvatarExperienceLandingPage`))); err != nil {
			  return false, errors.Wrap(err, "onAppReady was not found in LogCat")
		  }

		  return true, nil
	  })
}
