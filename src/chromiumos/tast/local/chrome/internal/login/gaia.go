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
	if err := oobeConn.Exec(ctx, "Oobe.skipToLoginForTesting()"); err != nil {
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
		if err := oobeConn.Exec(ctx, "Oobe.showAddUserForTesting()"); err != nil {
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

	// Fill in username.
	if err := insertGAIAField(ctx, gaiaConn, "#identifierId", cfg.User); err != nil {
		return errors.Wrap(err, "failed to fill username field")
	}
	if err := oobeConn.Exec(ctx, "Oobe.clickGaiaPrimaryButtonForTesting()"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	// Fill in password / contact email.
	authType, err := getAuthType(ctx, gaiaConn)
	if err != nil {
		return errors.Wrap(err, "could not determine the authentication type for this account")
	}
	if authType == config.PasswordAuth {
		testing.ContextLog(ctx, "This account uses password authentication")
		if cfg.Pass == "" {
			return errors.New("please supply a password with chrome.Auth()")
		}
		if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", cfg.Pass); err != nil {
			return errors.Wrap(err, "failed to fill in password field")
		}
	} else if authType == config.ContactAuth {
		testing.ContextLog(ctx, "This account uses contact email authentication")
		if cfg.Contact == "" {
			return errors.New("please supply a contact email with chrome.Contact()")
		}
		if err := insertGAIAField(ctx, gaiaConn, "input[name=email]", cfg.Contact); err != nil {
			return errors.Wrap(err, "failed to fill in contact email field")
		}
	} else {
		return errors.Errorf("got an invalid authentication type (%q) for this account", authType)
	}
	if err := oobeConn.Exec(ctx, "Oobe.clickGaiaPrimaryButtonForTesting()"); err != nil {
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
	if cfg.ParentUser != "" {
		if err := performUnicornParentLogin(ctx, cfg, sess, oobeConn, gaiaConn); err != nil {
			return err
		}
	}

	return nil
}

// getAuthType determines the authentication type by checking whether the current page
// is expecting a password or contact email input.
func getAuthType(ctx context.Context, gaiaConn *driver.Conn) (config.AuthType, error) {
	const query = `
	(function() {
		if (document.getElementById('password')) {
			return 'password';
		}
		if (document.getElementsByName('email').length > 0) {
			return 'contact';
		}
		return "";
	})();
	`
	t := config.UnknownAuth
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := gaiaConn.Eval(ctx, query, &t); err != nil {
			return err
		}
		if t == config.PasswordAuth || t == config.ContactAuth {
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
	script := fmt.Sprintf(`
		(function() {
			const field = document.querySelector(%q);
			field.value = %q;
		})()`, selector, value)
	if err := gaiaConn.Exec(ctx, script); err != nil {
		return errors.Wrapf(err, "failed to use %q element", selector)
	}
	return nil
}

// performUnicornParentLogin Logs in a parent account and accepts Unicorn permissions.
// This function is heavily based on NavigateUnicornLogin() in Catapult's
// telemetry/telemetry/internal/backends/chrome/oobe.py.
func performUnicornParentLogin(ctx context.Context, cfg *config.Config, sess *driver.Session, oobeConn, gaiaConn *driver.Conn) error {
	normalizedParentUser, err := session.NormalizeEmail(cfg.ParentUser, false)
	if err != nil {
		return errors.Wrapf(err, "failed to normalize email %q", cfg.User)
	}

	testing.ContextLogf(ctx, "Clicking button that matches parent email: %q", normalizedParentUser)
	buttonTextQuery := `
		(function() {
			const buttons = document.querySelectorAll('%[1]s');
			if (buttons === null){
				throw new Error('no buttons found on screen');
			}
			return [...buttons].map(button=>button.textContent);
		})();`

	clickButtonQuery := `
                (function() {
                        const buttons = document.querySelectorAll('%[1]s');
                        if (buttons === null){
                                throw new Error('no buttons found on screen');
                        }
                        for (const button of buttons) {
                                if (button.textContent.indexOf(%[2]q) !== -1) {
                                        button.click();
                                        return;
                                }
                        }
                        throw new Error(%[2]q + ' button not found');
                })();`
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var buttons []string
		if err := gaiaConn.Eval(ctx, fmt.Sprintf(buttonTextQuery, "[data-email]"), &buttons); err != nil {
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
			return gaiaConn.Exec(ctx, fmt.Sprintf(clickButtonQuery, "[data-email]", button))
		}
		return errors.New("no button matches email")
	}, pollOpts); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to select parent user")
	}

	testing.ContextLog(ctx, "Typing parent password")
	if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", cfg.ParentPass); err != nil {
		return err
	}
	if err := oobeConn.Exec(ctx, "Oobe.clickGaiaPrimaryButtonForTesting()"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	testing.ContextLog(ctx, "Accepting Unicorn permissions")
	clickAgreeQuery := fmt.Sprintf(clickButtonQuery, "button", "agree")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return gaiaConn.Exec(ctx, clickAgreeQuery)
	}, pollOpts); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to accept Unicorn permissions")
	}

	return nil
}
