// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckInstallPlay,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify the app has proper info",
		Contacts: []string{
			"jshikaram@chromium.org",
			"ashpakov@google.com", // until Sept 2022
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedWithPlayStoreEnabledPopup",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 5 * time.Minute,
	})
}

const (
	pkgName   = "com.potatoh.playbilling"
	appName   = "Web Play Billing Sample App"
	appID     = "hccligcafeiehpaolmeeialgndmkhojb"
	tryLimit  = 3
	uiTimeout = 30 * time.Second
)

// CheckInstallPlay Checks install play
func CheckInstallPlay(ctx context.Context, s *testing.State) {

	p := s.FixtValue().(*arc.PreData)
	a := p.ARC
	cr := p.Chrome
	d := p.UIDevice

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "CheckInstallPlay")

	// Install app.
	s.Logf("Installing %s", pkgName)
	if err := playstore.InstallApp(ctx, a, d, pkgName, tryLimit); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	ui := uiauto.New(tconn)

	if err := apps.Launch(ctx, tconn, appID); err != nil {
		s.Fatal("Failed to restart terminal app: ", err)
	}

	appConn, err := uIConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to connect to web page: ", err)
	}
	defer appConn.Close()

	s.Log("Check if logged in")
	profileMenuSelector := `document.getElementById('app-bar').shadowRoot.getElementById('profile-menu')`

	if err := appConn.WaitForExpr(ctx, profileMenuSelector); err != nil {
		s.Fatal("Failed to find profile-menu button: ", err)
	}

	// The code below could fail, if the sign in screen appears and quickly disappears.
	if err := ui.Retry(2, func(ctx context.Context) error {
		return signIn(ctx, appConn, profileMenuSelector, cr)
	})(ctx); err != nil {
		s.Fatal("Failed to sign in: ", err)
	}

	// TODO: instead wait until the products appear on screen
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Timed out on sleep: ", err)
	}

	s.Log("Read logs")
	var result []string
	if err := appConn.Eval(ctx,
		`document.getElementById('log-box').innerHTML.replace(/<br>\s*$/, '').split('    ')`,
		&result); err != nil {
		s.Fatal("Failed to get log-box: ", err)
	}

	type price struct {
		Currency string `json:"currency"`
		Value    string `json:"value"`
	}

	type getDetailsResult struct {
		Description       string `json:"description"`
		Title             string `json:"title"`
		ItemID            string `json:"itemId"`
		Price             price  `json:"price"`
		IntroductoryPrice price  `json:"introductoryPrice"`
		PurchaseType      string `json:"purchaseType"`
	}

	var detailsResult []getDetailsResult

	for _, v := range result {
		if strings.HasPrefix(v, "getDetails returned ") {
			if err := json.Unmarshal([]byte(strings.Trim(v, "getDetails returned ")), &detailsResult); err != nil {
				s.Fatal("Unable to parse json: ", err)
			}
		}
	}

	s.Logf("result: %#v", detailsResult)
}

func signIn(ctx context.Context, appConn *chrome.Conn, profileMenuSelector string, cr *chrome.Chrome) error {
	var loggedIn bool
	if err := appConn.Eval(ctx,
		fmt.Sprintf("%s.loggedIn", profileMenuSelector),
		&loggedIn); err != nil {
		return errors.Wrap(err, "failed to get loggedIn status")
	}

	if !loggedIn {
		//s.Log("Trigger sign in")
		if err := appConn.Eval(ctx,
			fmt.Sprintf("%s._requestSignIn()", profileMenuSelector),
			nil); err != nil {
			return errors.Wrap(err, "failed to click a login button")
		}

		ctxWithTimeout, cancel := context.WithTimeout(ctx, uiTimeout)
		defer cancel()

		targetURL := "https://accounts.google.com"
		signInConn, err := cr.NewConnForTarget(ctxWithTimeout, chrome.MatchTargetURLPrefix(targetURL))
		if err != nil {
			return errors.Wrap(err, "failed to get sign in connection")
		}
		if err := webutil.WaitForQuiescence(ctx, signInConn, uiTimeout); err != nil {
			return errors.Wrap(err, "failed to wait for WaitForQuiescence")
		}

		// s.Log("Click user account entry")
		if err := clickByJsSelector(ctx, signInConn, fmt.Sprintf(`document.querySelector("[data-email='%s']")`, cr.Creds().User)); err != nil {
			return errors.Wrap(err, "failed to click user account entry")
		}
	}

	return nil
}

// // WaitForCalculateResult waits until the calculation result is expected.
// func WaitForCalculateResult(appConn *chrome.Conn, expectedResult string) uiauto.Action {
// 	script := `document.querySelector(".calculator-display").innerText`
// 	var result string

// 	return action.RetrySilently(3, func(ctx context.Context) error {
// 		if err := appConn.Eval(ctx, script, &result); err != nil {
// 			return errors.Wrap(err, "failed to get calculation result")
// 		}
// 		if result != expectedResult {
// 			return errors.Errorf("Wrong calculation result: got %q; want %q", result, expectedResult)
// 		}
// 		return nil
// 	}, time.Second)
// }

func clickByJsSelector(ctx context.Context, appConn *chrome.Conn, jsExpr string) error {
	if err := appConn.WaitForExpr(ctx, jsExpr); err != nil {
		return errors.Wrapf(err, "failed to wait for %q to be defined", jsExpr)
	}
	if err := appConn.Eval(ctx, fmt.Sprintf("%s.click()", jsExpr), nil); err != nil {
		return errors.Wrapf(err, "failed to click %q", jsExpr)
	}

	return nil
}

func uIConn(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	// Establish a Chrome connection to the Calculator app and wait for it to finish loading.
	targetURL := "https://twa-sample-cros-sa.web.app/"
	appConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(targetURL))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to target %q", targetURL)
	}
	if err := webutil.WaitForQuiescence(ctx, appConn, 10*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait for Calculator app to finish loading")
	}
	return appConn, nil
}
