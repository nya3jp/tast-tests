// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dgapi2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/playbilling"
	"chromiumos/tast/testing"
)

// TestAppDgapi2 represents the Play Billing test PWA and ARC Payments Overlay.
type TestAppDgapi2 struct {
	appconn     *chrome.Conn
	arc         *arc.ARC
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
func NewTestAppDgapi2(ctx context.Context, cr *chrome.Chrome, d *ui.Device, tconn *chrome.TestConn, a *arc.ARC) (*TestAppDgapi2, error) {
	return &TestAppDgapi2{
		appconn:     nil,
		arc:         a,
		cr:          cr,
		tconn:       tconn,
		uiAutomator: d,
	}, nil
}

// InstallApp installs DGAPI2 test app.
func (ta *TestAppDgapi2) InstallApp(ctx context.Context) error {
	if err := playstore.InstallApp(ctx, ta.arc, ta.uiAutomator, pkgName, &playstore.Options{TryLimit: tryLimit}); err != nil {
		return errors.Wrapf(err, "failed to install app %q", pkgName)
	}

	return nil
}

// Launch starts a new TestAppDgapi2 window.
func (ta *TestAppDgapi2) Launch(ctx context.Context) error {
	return uiauto.Combine("launch test app",
		func(ctx context.Context) error {
			return apps.Launch(ctx, ta.tconn, appID)
		},
		func(ctx context.Context) error {
			appconn, err := ta.cr.NewConnForTarget(ctx, chrome.MatchTargetURL(targetURL))
			ta.appconn = appconn
			return err
		},
	)(ctx)
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
	return uiauto.Combine("sign in user",
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
	)(ctx)
}

func (ta *TestAppDgapi2) isSignedIn(ctx context.Context, appConn *chrome.Conn) (bool, error) {
	signedIn := false
	if err := appConn.Eval(ctx, profileMenuLoggedInJS, &signedIn); err != nil {
		return signedIn, errors.Wrap(err, "failed to get loggedIn status")
	}

	return signedIn, nil
}

// waitSignedInState wait until signed in state changes to expect one.
func (ta *TestAppDgapi2) waitSignedInState(ctx context.Context, expectedSignedInState bool) error {
	jsExpr := ""
	if expectedSignedInState {
		jsExpr = fmt.Sprintf("!!%s === true", profileMenuLoggedInJS)
	} else {
		jsExpr = fmt.Sprintf("!%s === true", profileMenuLoggedInJS)
	}

	return ta.appconn.WaitForExprWithTimeout(ctx, jsExpr, uiTimeout)
}

func (ta *TestAppDgapi2) signIn(ctx context.Context, appConn *chrome.Conn, cr *chrome.Chrome) error {
	var signInConn *chrome.Conn

	defer func() {
		if signInConn != nil {
			signInConn.Close()
		}
	}()

	userEntryJS := fmt.Sprintf(`document.querySelector("[data-email='%s']")`, cr.Creds().User)
	return uiauto.Combine("sign in the user",
		func(context.Context) error { return appConn.Eval(ctx, profileMenuSignInJS, nil) },
		func(context.Context) error {
			ctxWithTimeout, cancel := context.WithTimeout(ctx, uiTimeout)
			defer cancel()
			var err error
			signInConn, err = cr.NewConnForTarget(ctxWithTimeout, chrome.MatchTargetURLPrefix(accountURL))
			return err
		},
		func(context.Context) error {
			return signInConn.WaitForExprWithTimeout(ctx, userEntryJS, uiTimeout)
		},
		func(context.Context) error {
			// Sleep briefly because login button may not be clickable yet.
			return testing.Sleep(ctx, 1*time.Second)
		},
		func(context.Context) error {
			return signInConn.Eval(ctx, fmt.Sprintf("%s.click()", userEntryJS), nil)
		},
		func(context.Context) error {
			return ta.waitSignedInState(ctx, true)
		},
	)(ctx)
}

// SignOut signs out of the app.
func (ta *TestAppDgapi2) SignOut(ctx context.Context) error {
	return uiauto.Combine("sign out the user",
		func(context.Context) error {
			return ta.appconn.Eval(ctx, profileMenuSignOutJS, nil)
		},
		webutil.WaitForQuiescenceAction(ta.appconn, uiTimeout),
	)(ctx)
}

// verifyLogs retrieves logs, executes verifyFn on them and returns the result.
func (ta *TestAppDgapi2) verifyLogs(ctx context.Context, verifyFn func(logs []string) error) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		var logs []string
		if err := ta.appconn.Eval(ctx, logBoxLogLinesJS, &logs); err != nil {
			return errors.Wrap(err, "failed to retrieve logs")
		}

		return verifyFn(logs)
	}, &testing.PollOptions{Timeout: uiTimeout, Interval: time.Second})
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
	return ta.verifyLogs(ctx, func(logs []string) error {
		foundEntry := ""
		getDetailsPrefix := "getDetails returned "
		for _, v := range logs {
			if strings.HasPrefix(v, getDetailsPrefix) {
				foundEntry = v
			}
		}

		if foundEntry == "" {
			return errors.Errorf(`failed to find a log entry starting with "getDetails returned ", received: %q`, logs)
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
	})
}

// VerifyLogsContain verifies logs contain a specific entry.
func (ta *TestAppDgapi2) VerifyLogsContain(ctx context.Context, str string) error {
	return ta.verifyLogs(ctx, func(logs []string) error {
		isEntryFound := false
		for _, v := range logs {
			if v == str {
				isEntryFound = true
			}
		}

		if !isEntryFound {
			return errors.Errorf("unable to find a log entry starting with %q", str)
		}

		return nil
	})
}

// PurchaseOneTime purchases a onetime item.
func (ta *TestAppDgapi2) PurchaseOneTime(ctx context.Context) error {
	findItemJS := ta.skuSelectorByID(oneTimeID)
	purchaseItemButtonJS := fmt.Sprintf("%s.shadowRoot.querySelector('mwc-button')", findItemJS)

	return uiauto.Combine(fmt.Sprintf("purchase item with ID %q", oneTimeID),
		func(ctx context.Context) error {
			return ta.appconn.WaitForExprWithTimeout(ctx, findItemJS, uiTimeout)
		},
		playbilling.ClickElementByCDP(ta.appconn, purchaseItemButtonJS),
		playbilling.Click1TapBuy(ta.uiAutomator),
		playbilling.RequiredAuthConfirm(ta.uiAutomator),
		playbilling.TapPointsDecline(ta.uiAutomator),
		webutil.WaitForQuiescenceAction(ta.appconn, uiTimeout),
	)(ctx)
}

// TryPurchaseOneTimeItemSecondTime attempts to purchase a onetime item second time, fail and close the error window.
func (ta *TestAppDgapi2) TryPurchaseOneTimeItemSecondTime(ctx context.Context) error {
	findItemJS := ta.skuSelectorByID(oneTimeID)
	purchaseItemButtonJS := fmt.Sprintf("%s.shadowRoot.querySelector('mwc-button')", findItemJS)

	return uiauto.Combine(fmt.Sprintf("purchase item with ID %q second time", oneTimeID),
		func(ctx context.Context) error {
			return ta.appconn.WaitForExprWithTimeout(ctx, findItemJS, uiTimeout)
		},
		playbilling.ClickElementByCDP(ta.appconn, purchaseItemButtonJS),
		playbilling.AlreadyOwnErrorClose(ta.uiAutomator),
	)(ctx)
}

// TryConsumeOneTime consumes a onetime item.
func (ta *TestAppDgapi2) TryConsumeOneTime(ctx context.Context) error {
	findItemJS := ta.skuSelectorByID(oneTimeID)
	consumeItemJS := fmt.Sprintf("%s.consume()", findItemJS)

	return action.IfSuccessThen(
		func(ctx context.Context) error {
			isPurchased, err := ta.isPurchased(ctx, oneTimeID)
			if err != nil {
				return errors.Wrapf(err, "failed to get purchased status of item %s", findItemJS)
			}

			if !isPurchased {
				return errors.Wrapf(err, "item %s is not purchased - unable to consume it", oneTimeID)
			}

			return nil
		},
		uiauto.Combine("consume a onetime sku",
			func(context.Context) error {
				return ta.appconn.Eval(ctx, consumeItemJS, nil)
			},
			func(context.Context) error {
				return ta.waitPurchasedState(ctx, oneTimeID, false)
			},
		),
	)(ctx)
}

// isPurchased Checks if an item is purchased
func (ta *TestAppDgapi2) isPurchased(ctx context.Context, itemID string) (bool, error) {
	isPurchased := false
	if err := ta.appconn.Eval(ctx, ta.purchaseStatusByID(itemID), &isPurchased); err != nil {
		return isPurchased, errors.Wrapf(err, "failed to get purchased status of item %s", itemID)
	}

	return isPurchased, nil
}

// waitPurchasedState wait until an item purchased state changes to expect one.
func (ta *TestAppDgapi2) waitPurchasedState(ctx context.Context, itemID string, expectedPurchasedState bool) error {
	jsExpr := ""
	if expectedPurchasedState {
		jsExpr = fmt.Sprintf("!!%s === true", ta.purchaseStatusByID(itemID))
	} else {
		jsExpr = fmt.Sprintf("!%s === true", ta.purchaseStatusByID(itemID))
	}

	return ta.appconn.WaitForExprWithTimeout(ctx, jsExpr, uiTimeout)
}

func (ta *TestAppDgapi2) skuSelectorByID(itemID string) string {
	return fmt.Sprintf(`[...%s].find(i => i.details.itemId === "%s")`, itemsJS, oneTimeID)
}

func (ta *TestAppDgapi2) purchaseStatusByID(itemID string) string {
	// for purchase status definition see https://github.com/chromeos/pwa-play-billing/blob/main/src/js/components/sku-list.js#L117
	return fmt.Sprintf("!!%s.consume", ta.skuSelectorByID(itemID))
}
