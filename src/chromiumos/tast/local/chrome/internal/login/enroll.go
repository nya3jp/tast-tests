// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

//  domainRe is a regex used to obtain the domain (without top level domain) out of an email string.
//  e.g. a@managedchrome.com -> [a@managedchrome.com managedchrome] and
//  ex2@domainp1.domainp2.com -> [ex2@domainp1.domainp2.com domainp1.domainp2]
var domainRe = regexp.MustCompile(`^[^@]+@([^@]+)\.[^.@]*$`)

//  fullDomainRe is a regex used to obtain the full domain (with top level domain) out of an email string.
//  e.g. a@managedchrome.com -> [a@managedchrome.com managedchrome.com] and
//  ex2@domainp1.domainp2.com -> [ex2@domainp1.domainp2.com domainp1.domainp2.com]
var fullDomainRe = regexp.MustCompile(`^[^@]+@([^@]+)$`)

// userDomain will return the "domain" section (without top level domain) of user.
// e.g. something@managedchrome.com will return "managedchrome"
// or x@domainp1.domainp2.com would return "domainp1domainp2"
func userDomain(user string) (string, error) {
	m := domainRe.FindStringSubmatch(user)
	// This check mandates the same format as the fake DM server.
	if len(m) != 2 {
		return "", errors.New("'user' must have exactly 1 '@' and atleast one '.' after the @")
	}
	return strings.Replace(m[1], ".", "", -1), nil
}

// fullUserDomain will return the full "domain" (including top level domain) of user.
// e.g. something@managedchrome.com will return "managedchrome.com"
// or x@domainp1.domainp2.com would return "domainp1.domainp2.com"
func fullUserDomain(user string) (string, error) {
	m := fullDomainRe.FindStringSubmatch(user)
	// If nothing is returned, the enrollment will fail.
	if len(m) != 2 {
		return "", errors.New("'user' must have exactly 1 '@'")
	}
	return m[1], nil
}

// enterpriseEnrollTargets returns the Gaia WebView targets, which are used
// to help enrollment on the device.
// Returns nil if none are found.
func enterpriseEnrollTargets(ctx context.Context, sess *driver.Session, userDomain string) ([]*driver.Target, error) {
	isGAIAWebView := func(t *driver.Target) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	}

	targets, err := sess.FindTargets(ctx, isGAIAWebView)
	if err != nil {
		return nil, err
	}

	// It's common for multiple targets to be returned.
	// We want to run the command specifically on the "apps" target.
	var enterpriseTargets []*driver.Target
	for _, target := range targets {
		u, err := url.Parse(target.URL)
		if err != nil {
			continue
		}

		q := u.Query()
		clientID := q.Get("client_id")
		managedDomain := q.Get("manageddomain")
		flowName := q.Get("flowName")

		if clientID != "" && managedDomain != "" && flowName != "" {
			if strings.Contains(clientID, "apps.googleusercontent.com") &&
				strings.Contains(managedDomain, userDomain) &&
				strings.Contains(flowName, "SetupChromeOs") {
				enterpriseTargets = append(enterpriseTargets, target)
			}
		}
	}

	return enterpriseTargets, nil
}

// waitForEnrollmentLoginScreen will wait for the Enrollment screen to complete
// and the Enrollment login screen to appear. If the login screen does not appear
// the testing.Poll will timeout.
func waitForEnrollmentLoginScreen(ctx context.Context, cfg *config.Config, sess *driver.Session) error {
	testing.ContextLog(ctx, "Waiting for enrollment to complete")
	user := cfg.EnrollmentCreds().User

	fullDomain, err := fullUserDomain(user)
	if err != nil {
		return errors.Wrap(err, "no valid full user domain found")
	}
	loginBanner := fmt.Sprintf(`document.querySelectorAll('span[title=%q]').length;`,
		fullDomain)

	userDomain, err := userDomain(user)
	if err != nil {
		return errors.Wrap(err, "no vaid user domain found")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		gaiaTargets, err := enterpriseEnrollTargets(ctx, sess, userDomain)
		if err != nil {
			return errors.Wrap(err, "no Enrollment webview targets")
		}
		for _, gaiaTarget := range gaiaTargets {
			webViewConn, err := sess.NewConnForTarget(ctx, driver.MatchTargetURL(gaiaTarget.URL))
			if err != nil {
				// If an error occurs during connection, continue to try.
				// Enrollment will only proceed if the eval below succeeds.
				continue
			}
			defer webViewConn.Close()
			content := -1
			if err := webViewConn.Eval(ctx, loginBanner, &content); err != nil {
				return err
			}
			// Found the login screen.
			if content == 1 {
				testing.ContextLogf(ctx, "find login target: %s, %s", gaiaTarget.TargetID, gaiaTarget.URL)
				return nil
			}
		}
		return errors.New("Enterprise Enrollment login screen not found")
	}, &testing.PollOptions{Timeout: 45 * time.Second}); err != nil {
		return err
	}

	return nil
}

// performFakeEnrollment will perform enterprise enrollment with a fake, local device management server and wait for it to complete.
func performFakeEnrollment(ctx context.Context, cfg *config.Config, sess *driver.Session) error {
	ctx, st := timing.Start(ctx, "enroll")
	defer st.End()

	conn, err := WaitForOOBEConnection(ctx, sess)
	if err != nil {
		return errors.Wrap(err, "could not find OOBE connection for enrollment")
	}
	defer conn.Close()

	creds := cfg.EnrollmentCreds()
	testing.ContextLogf(ctx, "Performing enrollment with %s", creds.User)
	if err := conn.Call(ctx, nil, "Oobe.loginForTesting", creds.User, creds.Pass, creds.GAIAID, true); err != nil {
		return errors.Wrap(err, "failed to trigger enrollment")
	}

	if err := waitForEnrollmentLoginScreen(ctx, cfg, sess); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "could not enroll")
	}

	return nil
}

// performEnrollment will enterprise enroll the test device using the OOBE screen.
func performEnrollment(ctx context.Context, cfg *config.Config, sess *driver.Session) error {
	ctx, st := timing.Start(ctx, "enroll")
	defer st.End()

	conn, err := WaitForOOBEConnection(ctx, sess)
	if err != nil {
		return errors.Wrap(err, "could not find OOBE connection for enrollment")
	}
	defer conn.Close()

	creds := cfg.EnrollmentCreds()
	testing.ContextLogf(ctx, "Performing enrollment with %s", creds.User)

	// Enterprise enrollment requires Internet connectivity.
	if err := shill.WaitForOnline(ctx); err != nil {
		return err
	}

	if err := conn.Call(ctx, nil, "Oobe.skipToLoginForTesting"); err != nil {
		return err
	}

	if err := conn.WaitForExpr(ctx, "OobeAPI.screens.GaiaScreen.isVisible()"); err != nil {
		return errors.Wrap(err, "failed to wait for the OOBE Gaia sign in screen")
	}

	if err := conn.Call(ctx, nil, "Oobe.switchToEnterpriseEnrollmentForTesting"); err != nil {
		return err
	}

	if err := performEnrollmentSignIn(ctx, conn, creds, sess, 5); err != nil {
		return err
	}

	if err := waitForEnrollmentLoginScreen(ctx, cfg, sess); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "could not enroll")
	}

	return nil
}

func performEnrollmentSignIn(ctx context.Context, oobeConn *driver.Conn, creds config.Creds, sess *driver.Session, retries uint) error {
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.signInScreen.isVisible()"); err != nil {
		return errors.Wrap(err, "failed to wait for the OOBE enterprise enrollment signin screen to be ready")
	}

	// Get GaiaConn for automating login on the enrollment screen
	isGAIAWebView := func(t *driver.Target) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	}

	testing.ContextLog(ctx, "Waiting for GAIA webview")
	var enterpriseTarget *driver.Target
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		targets, err := sess.FindTargets(ctx, isGAIAWebView)
		if err != nil {
			return err
		}
		for _, target := range targets {
			u, err := url.Parse(target.URL)
			if err != nil {
				continue
			}

			q := u.Query()
			flow := q.Get("flow")

			if flow == "enterprise" {
				enterpriseTarget = target
				return nil
			}
		}
		return errors.New("could not find the enterprise enrollment Gaia login webview")

	}, pollOpts); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "GAIA webview not found")
	}

	gaiaConn, err := sess.NewConnForTarget(ctx, driver.MatchTargetID(enterpriseTarget.TargetID))
	if err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to connect to GAIA webview")
	}
	defer gaiaConn.Close()

	testing.ContextLog(ctx, "Type user name")
	if err := insertGAIAField(ctx, gaiaConn, "#identifierId", creds.User); err != nil {
		return errors.Wrap(err, "failed to fill username field")
	}

	if err := oobeConn.WaitForExprFailOnErrWithTimeout(ctx,
		"OobeAPI.screens.EnterpriseEnrollmentScreen.signInScreen.waitAndClickNext()",
		3*time.Second); err != nil {
		return errors.Wrap(err, "failed to click the next button")
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.signInScreen.isVisible()"); err != nil {
		return errors.Wrap(err, "failed to wait for the OOBE enterprise enrollment signin screen to be ready")
	}

	testing.ContextLog(ctx, "Type password")
	if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", creds.Pass); err != nil {
		return errors.Wrap(err, "failed to fill in password field")
	}

	testing.ContextLog(ctx, "Click Next")

	if err := oobeConn.WaitForExprFailOnErrWithTimeout(ctx,
		"OobeAPI.screens.EnterpriseEnrollmentScreen.signInScreen.waitAndClickNext()",
		3*time.Second); err != nil {
		return errors.Wrap(err, "failed to click the next button")
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx,
		"!OobeAPI.screens.EnterpriseEnrollmentScreen.isEnrollmentInProgress()"); err != nil {
		return errors.Wrap(err, "failed to wait for enrollment to complete")
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx,
		"OobeAPI.screens.EnterpriseEnrollmentScreen.successScreen.isVisible()"); err != nil {
		// Sometimes enrollment may fail due to one-off issues with the device management server.
		// Check if enrollment maybe retried.

		var isOnErrorStep bool
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.errorScreen.isVisible()", &isOnErrorStep); err != nil {
			return errors.Wrap(err, "failed to check enrollment step")
		}

		if !isOnErrorStep {
			return errors.New("unexpected step after enrollment signin failure")
		}

		var canRetry bool
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.errorScreen.canRetryEnrollment()", &canRetry); err != nil {
			return errors.Wrap(err, "failed to check enrollment step")
		}

		if !canRetry {
			var enrollmentErrorMsg string
			if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.errorScreen.getErrorMsg()", &enrollmentErrorMsg); err != nil {
				return errors.Wrap(err, "failed to get unretriable enrollment error msg")
			}
			return errors.Errorf("enrollment hit an unrecoverable error: %v", enrollmentErrorMsg)
		}

		retries--
		if retries == 0 {
			return errors.Wrap(err, "Exhausted retries")
		}

		if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.errorScreen.clickRetryButton()", nil); err != nil {
			return errors.Wrap(err, "failed to click the retry button")
		}

		return performEnrollmentSignIn(ctx, oobeConn, creds, sess, retries)
	}

	if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.successScreen.clickNext()", nil); err != nil {
		return errors.Wrap(err, "failed to click the enrollment done button")
	}

	return nil
}
