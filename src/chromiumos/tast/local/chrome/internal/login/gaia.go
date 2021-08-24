// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

// localPassword is used in OOBE login screen. When contact email approval flow is used,
// there is no password supplied by the user and this local password will be used to encrypt
// cryptohome instead.
const localPassword = "test0000"

// performGAIALogin waits for and interacts with the GAIA webview to perform login.
// This function is heavily based on NavigateGaiaLogin() in Catapult's
// telemetry/telemetry/internal/backends/chrome/oobe.py.
func performGAIALogin(ctx context.Context, cfg *config.Config, sess *driver.Session, oobeConn *driver.Conn) error {
	if err := oobeConn.Call(ctx, nil, "Oobe.skipToLoginForTesting"); err != nil {
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

	isGAIAWebView := func(t *driver.Target) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	}

	testing.ContextLog(ctx, "Waiting for GAIA webview")
	var target *driver.Target
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if targets, err := sess.FindTargets(ctx, isGAIAWebView); err != nil {
			return err
		} else if len(targets) != 1 {
			return errors.Errorf("got %d GAIA targets; want 1", len(targets))
		} else {
			target = targets[0]
			return nil
		}
	}, pollOpts); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "GAIA webview not found")
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

		authType, err = getAuthType(ctx, gaiaConn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "could not determine the authentication type for this account"))
		}

		if authType != config.RetryAuth {
			return nil
		}

		testing.ContextLog(ctx, "Go back and try to sign in again")
		// Click button to go back to previous step.
		if err := oobeConn.Call(ctx, nil, "Oobe.clickGaiaPrimaryButtonForTesting"); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to click the primary action button to go back to the previous page"))
		}
		return errors.New("couldn't sign the user in")
	}, pollOpts); err != nil {
		return err
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
// If the process gets stuck at the retry screen, it returns RetryAuth type so the login
// procedure can be retried.
func getAuthType(ctx context.Context, gaiaConn *driver.Conn) (config.AuthType, error) {
	const query = `
	(function() {
		if (document.getElementById('password')) {
			return 'password';
		}
		if (document.getElementsByName('email').length > 0) {
			return 'contact';
		}
		var targets = document.querySelectorAll('div.PrDSKc');
		if (targets.length === 2 && targets[1].innerText === 'Try using a different browser. If you’re already using a supported browser, you can try again to sign in.') {
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
		if t == config.PasswordAuth || t == config.ContactAuth || t == config.RetryAuth {
			return nil
		}
		return errors.New("failed to locate password or contact input field")
	}, pollOpts); err != nil {
		return config.UnknownAuth, err
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
