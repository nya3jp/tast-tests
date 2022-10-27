// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

// localPassword is used in OOBE login screen. When contact email approval flow is used,
// there is no password supplied by the user and this local password will be used to encrypt
// cryptohome instead.
const localPassword = "test0000"

// errLoginRetry is used to indicate the GAIA login procedure is currently
// at the retry page and should be retried.
var errLoginRetry = errors.New("login needs retry")

// Prefix of the prod GAIA sign in URL.
const prodGAIASignInURLPrefix = "https://accounts.google.com/"

// Prefix of the staging GAIA sign in URL.
const stagingGAIASignInURLPrefix = "https://gaiastaging.corp.google.com/"

// Prefix of the sandbox GAIA sign in URL.
const sandboxGAIASignInURLPrefix = "https://accounts.sandbox.google.com/"

// isGAIASignInURL checks if the given URL string is for GAIA sign in.
func isGAIASignInURL(u string) bool {
	return strings.HasPrefix(u, prodGAIASignInURLPrefix) ||
		strings.HasPrefix(u, stagingGAIASignInURLPrefix) ||
		strings.HasPrefix(u, sandboxGAIASignInURLPrefix)
}

// waitForSingleGAIAWebView waits until it finds a matching WebView target with
// the specified TargetMatcher function, or until timeout. Returns an error if
// the TargetMatcher finds more than one target matching the requirements.
// Used by automation to identify the correct GAIA WebView targets on the
// ChromeOS oobe. ChromeOS oobe typically have multiple different GAIA WebView
// targets simultaneously.
func waitForSingleGAIAWebView(ctx context.Context, sess *driver.Session, targetMatcher cdputil.TargetMatcher, timeout time.Duration) (*driver.Target, error) {
	testing.ContextLog(ctx, "Waiting for GAIA webview")
	po := &testing.PollOptions{Timeout: timeout}
	var target *driver.Target
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if targets, err := sess.FindTargets(ctx, targetMatcher); err != nil {
			return err
		} else if len(targets) != 1 {
			return errors.Errorf("got %d GAIA targets; want 1", len(targets))
		} else {
			target = targets[0]
			return nil
		}
	}, po); err != nil {
		return nil, errors.Wrap(sess.Watcher().ReplaceErr(err),
			"GAIA webview not found")
	}

	return target, nil
}

// MatchSignInGAIAWebView returns a function that matches GAIA sign in webview
// targets. The strategy for identifying a GAIA sign in target is copied from
// the Catapult telemetry project's oobe.py script.
func MatchSignInGAIAWebView(ctx context.Context, sess *driver.Session) cdputil.TargetMatcher {
	return func(t *driver.Target) bool {
		if t.Type != "webview" || !isGAIASignInURL(t.URL) {
			return false
		}

		gaiaConn, err := sess.NewConnForTarget(ctx, driver.MatchTargetID(t.TargetID))
		if err != nil {
			return false
		}
		defer gaiaConn.Close()

		isGAIA := false
		jsEval := fmt.Sprintf(`
			(function () {
				bases = document.getElementsByTagName('base');
				if (bases.length == 0) {
					return false;
				}
				href = bases[0].href;
				return (href.indexOf(%q) == 0 ||
					href.indexOf(%q) == 0 ||
					href.indexOf(%q) == 0);
			})()
		`, prodGAIASignInURLPrefix, stagingGAIASignInURLPrefix, sandboxGAIASignInURLPrefix)
		if err = gaiaConn.Eval(ctx, jsEval, &isGAIA); err != nil {
			return false
		}

		return isGAIA
	}
}

// performGAIALogin waits for and interacts with the GAIA webview to perform login.
// This function is heavily based on NavigateGaiaLogin() in Catapult's
// telemetry/telemetry/internal/backends/chrome/oobe.py.
func performGAIALogin(ctx context.Context, cfg *config.Config, sess *driver.Session, oobeConn *driver.Conn) error {
	if err := oobeConn.Call(ctx, nil, "OobeAPI.skipToLoginForTesting"); err != nil {
		return err
	}

	var url string
	if err := oobeConn.Eval(ctx, "window.location.href", &url); err != nil {
		return err
	}
	if strings.HasPrefix(url, "chrome://oobe/gaia-signin") {
		// Force show GAIA webview even if the cryptohome exists. When there is an existing
		// user on the device, the login screen would be chrome://oobe/gaia-signin instead
		// of the accounts.google.com webview. Use Oobe.showAddUserForTesting() to open that
		// webview so we can reuse the same login logic below.
		testing.ContextLogf(ctx, "Found %s, force opening GAIA webview", url)
		if err := oobeConn.Call(ctx, nil, "Oobe.showAddUserForTesting"); err != nil {
			return err
		}

		// If user creation screen asks whether to add an account for "You" or "A child",
		// click next button to choose "You".
		testing.ContextLog(ctx, "Clicking next button on user creation screen")
		err := oobeConn.Call(ctx, nil, `() => {
		  const elem = document.querySelector('user-creation-element');
		  if (!elem || !elem.shadowRoot) {
		    // This is not an error because user creation screen is not always shown.
		    return;
		  }
		  const nextButton = elem.shadowRoot.querySelector('oobe-next-button');
		  if (!nextButton) {
		    throw new Error('Next button not found on user creation screen');
		  }
		  nextButton.click();
		}`)
		if err != nil {
			return err
		}
	}

	target, err := waitForSingleGAIAWebView(ctx, sess, MatchSignInGAIAWebView(ctx, sess), pollOpts.Timeout)
	if err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to find GAIA webview")
	}

	gaiaConn, err := sess.NewConnForTarget(ctx, driver.MatchTargetID(target.TargetID))
	if err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to connect to GAIA webview")
	}
	defer gaiaConn.Close()

	testing.ContextLog(ctx, "Performing GAIA login")
	creds := cfg.Creds()

	var authType config.AuthType
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Fill in username.
		if err := insertGAIAField(ctx, gaiaConn, "#identifierId", creds.User); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to fill username field"))
		}
		if err := oobeConn.Call(ctx, nil, "Oobe.clickGaiaPrimaryButtonForTesting"); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to click on the primary action button"))
		}

		if cfg.LoginMode() == config.SAMLLogin {
			if err := gaiaConn.WaitForExpr(ctx, "document.querySelector('title').innerHTML != 'Sign in - Google Accounts'"); err != nil {
				return errors.Wrap(err, "failed to wait for SAML page to be loaded")
			}
			return nil
		}

		authType, err = getAuthType(ctx, gaiaConn)
		if err == nil {
			return nil
		}
		if err != errLoginRetry {
			return testing.PollBreak(errors.Wrap(err, "could not determine the authentication type for this account"))
		}
		testing.ContextLog(ctx, "Try to sign in again")
		// Click "Try again" button to go to sign-in screen.
		if err := oobeConn.Call(ctx, nil, "Oobe.clickGaiaPrimaryButtonForTesting"); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to click the primary action button to go to sign-in page when retrying login"))
		}
		return errors.New("couldn't sign the user in and login retry is needed")
	}, pollOpts); err != nil {
		return err
	}

	// Skip authentication for SAML flows and give back the current context.
	if cfg.LoginMode() == config.SAMLLogin {
		return nil
	}

	// Fill in password / contact email.
	if authType == config.PasswordAuth {
		testing.ContextLog(ctx, "This account uses password authentication")
		if creds.Pass == "" {
			return errors.New("please supply a password with chrome.Auth()")
		}
		if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", creds.Pass); err != nil {
			return errors.Wrap(err, "failed to fill in password field")
		}
	} else if authType == config.ContactAuth {
		testing.ContextLog(ctx, "This account uses contact email authentication")
		if creds.Contact == "" {
			return errors.New("please supply a contact email with chrome.Contact()")
		}
		if err := insertGAIAField(ctx, gaiaConn, "input[name=email]", creds.Contact); err != nil {
			return errors.Wrap(err, "failed to fill in contact email field")
		}
	} else {
		return errors.Errorf("got an invalid authentication type (%q) for this account", authType)
	}
	if err := oobeConn.Call(ctx, nil, "Oobe.clickGaiaPrimaryButtonForTesting"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	// Wait for contact email approval and fill in local password.
	if authType == config.ContactAuth {
		testing.ContextLog(ctx, "Please go to https://g.co/verifyaccount to approve the login request")
		testing.ContextLog(ctx, "Waiting for approval")
		if err := oobeConn.WaitForExpr(ctx, "OobeAPI.screens.ConfirmSamlPasswordScreen.isVisible()"); err != nil {
			return errors.Wrap(err, "failed to wait for OOBE password screen")
		}
		testing.ContextLog(ctx, "The login request is approved. Entering local password")
		if err := oobeConn.Call(ctx, nil, `(pw) => { OobeAPI.screens.ConfirmSamlPasswordScreen.enterManualPasswords(pw); }`, localPassword); err != nil {
			return errors.Wrap(err, "failed to fill in local password field")
		}
	}

	// Perform Unicorn login if parent user given.
	if creds.ParentUser != "" {
		if err := performUnicornParentLogin(ctx, cfg, sess, oobeConn, gaiaConn); err != nil {
			return err
		}
	}

	return nil
}

// getAuthType determines the authentication type by checking whether the current page
// is expecting a password or contact email input.
// If the current page is at the login retry screen, it returns errLoginRetry error.
func getAuthType(ctx context.Context, gaiaConn *driver.Conn) (config.AuthType, error) {
	const retry = "retry"

	const query = `
	(function() {
		if (document.getElementById('password')) {
			return 'password';
		}
		if (document.getElementsByName('email').length > 0) {
			return 'contact';
		}
		if (document.querySelector('div[data-primary-action-label="Try again"]')) {
			return 'retry';
		}
		return "";
	})();
	`
	t := config.UnknownAuth
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := gaiaConn.Eval(ctx, query, &t); err != nil {
			return err
		}
		if t == config.PasswordAuth || t == config.ContactAuth || t == retry {
			return nil
		}
		return errors.New("failed to locate password or contact input field")
	}, pollOpts); err != nil {
		return config.UnknownAuth, err
	}

	if t == retry {
		return config.UnknownAuth, errLoginRetry
	}
	return t, nil
}

// insertGAIAField fills a field of the GAIA login form.
func insertGAIAField(ctx context.Context, gaiaConn *driver.Conn, selector, value string) error {
	// Ensure that the input exists.
	if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
		"document.querySelector(%q)", selector)); err != nil {
		return errors.Wrapf(err, "failed to wait for %q element", selector)
	}
	// Ensure the input field is empty.
	// This confirms that we are not using the field before it is cleared.
	fieldReady := fmt.Sprintf(`
		(function() {
			const field = document.querySelector(%q);
			return field.value === "";
		})()`, selector)
	if err := gaiaConn.WaitForExpr(ctx, fieldReady); err != nil {
		return errors.Wrapf(err, "failed to wait for %q element to be empty", selector)
	}

	// Fill the field with value.
	if err := gaiaConn.Call(ctx, nil, `(selector, value) => {
	  const field = document.querySelector(selector);
	  field.value = value;
	}`, selector, value); err != nil {
		return errors.Wrapf(err, "failed to use %q element", selector)
	}
	return nil
}

// performUnicornParentLogin Logs in a parent account and accepts Unicorn permissions.
// This function is heavily based on NavigateUnicornLogin() in Catapult's
// telemetry/telemetry/internal/backends/chrome/oobe.py.
func performUnicornParentLogin(ctx context.Context, cfg *config.Config, sess *driver.Session, oobeConn, gaiaConn *driver.Conn) error {
	creds := cfg.Creds()

	normalizedParentUser, err := session.NormalizeEmail(creds.ParentUser, false)
	if err != nil {
		return errors.Wrapf(err, "failed to normalize email %q", creds.ParentUser)
	}

	testing.ContextLogf(ctx, "Clicking button that matches parent email: %q", normalizedParentUser)
	findButtonText := func(ctx context.Context, selector string) ([]string, error) {
		var ret []string
		err := gaiaConn.Call(ctx, &ret, `(selector) => {
		  const buttons = document.querySelectorAll(selector);
		  if (buttons === null){
		    throw new Error('no buttons found on screen');
		  }
		  return [...buttons].map(button=>button.textContent);
		}`, selector)
		return ret, err
	}

	clickButton := func(ctx context.Context, selector, text string) error {
		return gaiaConn.Call(ctx, nil, `(selector, text) => {
		  const buttons = document.querySelectorAll(selector);
		  if (buttons === null){
		    throw new Error('no buttons found on screen');
		  }
		  for (const button of buttons) {
		    if (button.textContent.indexOf(text) !== -1) {
		      button.click();
		      return;
		    }
		  }
		  throw new Error(text + ' button not found');
		}`, selector, text)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		buttons, err := findButtonText(ctx, "[data-email]")
		if err != nil {
			return err
		}
	NextButton:
		for _, button := range buttons {
			if len(button) < len(normalizedParentUser) {
				continue NextButton
			}
			// The end of button text contains the email.
			// Trim email to be the same length as normalizedParentUser.
			potentialEmail := button[len(button)-len(normalizedParentUser):]

			// Compare email to parent.
			for i := range normalizedParentUser {
				// Ignore wildcards.
				if potentialEmail[i] == '*' {
					continue
				}
				if potentialEmail[i] != normalizedParentUser[i] {
					continue NextButton
				}
			}

			// Button matches. Click it.
			return clickButton(ctx, "[data-email]", button)
		}
		return errors.New("no button matches email")
	}, pollOpts); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to select parent user")
	}

	testing.ContextLog(ctx, "Typing parent password")
	if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", creds.ParentPass); err != nil {
		return err
	}
	if err := oobeConn.Call(ctx, nil, "Oobe.clickGaiaPrimaryButtonForTesting"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	testing.ContextLog(ctx, "Accepting Unicorn permissions")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return clickButton(ctx, "button", "agree")
	}, pollOpts); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to accept Unicorn permissions")
	}

	return nil
}
