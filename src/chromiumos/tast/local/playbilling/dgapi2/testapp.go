// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dgapi2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/webutil"
)

// TestAppDgapi2 represents the Play Billing test PWA and ARC Payments Overlay.
type TestAppDgapi2 struct {
	appconn     *chrome.Conn
	cr          *chrome.Chrome
	tconn       *chrome.TestConn
	uiAutomator *ui.Device
}

// price represents price type used by DGAPI2 test app.
type price struct {
	Currency string `json:"currency"`
	Value    string `json:"value"`
}

// skuDetails represents sku details used by DGAPI2 test app.
type skuDetails struct {
	Description       string `json:"description"`
	Title             string `json:"title"`
	ItemID            string `json:"itemId"`
	Price             price  `json:"price"`
	IntroductoryPrice price  `json:"introductoryPrice"`
	PurchaseType      string `json:"purchaseType"`
}

// NewTestAppDgapi2 returns a reference to a new DGAPI2 Test App.
func NewTestAppDgapi2(ctx context.Context, cr *chrome.Chrome, d *ui.Device, tconn *chrome.TestConn) (*TestAppDgapi2, error) {
	return &TestAppDgapi2{
		cr:          cr,
		tconn:       tconn,
		appconn:     nil,
		uiAutomator: d,
	}, nil
}

// Launch starts a new TestAppDgapi2 window.
func (ta *TestAppDgapi2) Launch(ctx context.Context) error {
	if err := uiauto.Combine("launch",
		func(ctx context.Context) error {
			return apps.Launch(ctx, ta.tconn, appID)
		},
		func(ctx context.Context) error {
			appconn, err := ta.cr.NewConnForTarget(ctx, chrome.MatchTargetURL(targetURL))
			ta.appconn = appconn
			return err
		},
		func(ctx context.Context) error {
			return webutil.WaitForQuiescence(ctx, ta.appconn, uiTimeout)
		},
	)(ctx); err != nil {
		return errors.Wrapf(err, "failed to launch %q", appName)
	}

	return nil
}

// Close opposite of Launch, closes TestAppDgapi2 window and existing connections.
func (ta *TestAppDgapi2) Close(ctx context.Context) error {
	if err := apps.Close(ctx, ta.tconn, appID); err != nil {
		return errors.Wrapf(err, "failed to close app ID %q", appID)
	}

	if err := ta.appconn.Close(); err != nil {
		return errors.Wrap(err, "failed to close chrome connection")
	}

	ta.appconn = nil

	return nil
}

// SignIn logs into the app.
func (ta *TestAppDgapi2) SignIn(ctx context.Context) error {
	if err := uiauto.Combine("sign in user",
		func(ctx context.Context) error {
			return ta.appconn.WaitForExprWithTimeout(ctx, profileMenuJS, uiTimeout)
		},
		// TODO(b/224884883): The ta.signIn could fail, if the sign in screen appears, automatically logs user in and quickly disappears.
		// Therefore may need to retry it once.
		uiauto.Retry(2,
			func(ctx context.Context) error {
				if isSignedIn, err := ta.isSignedIn(ctx, ta.appconn); err != nil {
					return err
				} else if !isSignedIn {
					return ta.signIn(ctx, ta.appconn, ta.cr)
				}
				return nil
			},
		),
		func(context.Context) error {
			return webutil.WaitForQuiescence(ctx, ta.appconn, uiTimeout)
		},
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to sign in")
	}

	return nil
}

func (ta *TestAppDgapi2) isSignedIn(ctx context.Context, appConn *chrome.Conn) (bool, error) {
	var signedIn bool
	if err := appConn.Eval(ctx, profileMenuLoggedInJS, &signedIn); err != nil {
		return signedIn, errors.Wrap(err, "failed to get loggedIn status")
	}

	return signedIn, nil
}

func (ta *TestAppDgapi2) signIn(ctx context.Context, appConn *chrome.Conn, cr *chrome.Chrome) error {
	var signInConn *chrome.Conn = nil

	defer func() {
		if signInConn != nil {
			signInConn.Close()
		}
	}()

	userEntryJS := fmt.Sprintf(`document.querySelector("[data-email='%s']")`, cr.Creds().User)
	if err := uiauto.Combine("Sign in user",
		func(context.Context) error { return appConn.Eval(ctx, profileMenuSignInJS, nil) },
		func(context.Context) error {
			var err error
			ctxWithTimeout, cancel := context.WithTimeout(ctx, uiTimeout)
			defer cancel()
			signInConn, err = cr.NewConnForTarget(ctxWithTimeout, chrome.MatchTargetURLPrefix(accountURL))
			return err
		},
		func(context.Context) error {
			return webutil.WaitForQuiescence(ctx, signInConn, uiTimeout)
		},
		func(context.Context) error {
			return signInConn.WaitForExprWithTimeout(ctx, userEntryJS, uiTimeout)
		},
		func(context.Context) error {
			return signInConn.Eval(ctx, fmt.Sprintf("%s.click()", userEntryJS), nil)
		},
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to sign in the user")
	}

	return nil
}

// getLogs returns logs.
func (ta *TestAppDgapi2) getLogs(ctx context.Context) ([]string, error) {
	var result []string
	if err := ta.appconn.Eval(ctx, logBoxLogLinesJS, &result); err != nil {
		return nil, errors.Wrap(err, "failed to get logs")
	}
	return result, nil
}

func isItemValid(r skuDetails) bool {
	return r.ItemID != "" && r.Title != "" && r.Price.Currency != "" && r.Price.Value != ""
}

func all(vs []skuDetails, f func(skuDetails) bool) bool {
	for _, v := range vs {
		if !f(v) {
			return false
		}
	}
	return true
}

// VerifyDetailsLogs verifies logs contain expected getDetails response.
func (ta *TestAppDgapi2) VerifyDetailsLogs(ctx context.Context) error {
	logs, err := ta.getLogs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read logs")
	}

	var foundEntry string
	getDetailsPrefix := "getDetails returned "
	for _, v := range logs {
		if strings.HasPrefix(v, getDetailsPrefix) {
			foundEntry = v
		}
	}

	if foundEntry == "" {
		return errors.New(`unable to find a log entry starting with "getDetails returned "`)
	}

	var detailsResult []skuDetails
	if err := json.Unmarshal([]byte(strings.Trim(foundEntry, getDetailsPrefix)), &detailsResult); err != nil {
		return errors.Wrap(err, "unable to parse json")
	}

	areItemsValid := all(detailsResult, isItemValid)

	if !areItemsValid {
		return errors.Errorf("returned json items aren't valid: %v", detailsResult)
	}
	return nil
}
