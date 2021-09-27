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
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// maxGAIAEnterpriseEnrollmentRetries is the maximum number of times to retry
// enrollment.
// Enterprise enrollment may fail as a result of temporary server issues. Such
// failures are recoverable. To bypass these one-off failures, automation
// functions will re-attempt enrollment until consistent failure.
const maxGAIAEnterpriseEnrollmentRetries = 3

// enrollmentTimeout is the maximum amount of time to wait for enrollment to
// succeed.
const enrollmentTimeout = 3 * time.Minute

//  domainRe is a regex used to obtain the domain (without top level domain)
//  out of an email string.
//  e.g. a@managedchrome.com -> [a@managedchrome.com managedchrome] and
//  ex2@domainp1.domainp2.com -> [ex2@domainp1.domainp2.com domainp1.domainp2]
var domainRe = regexp.MustCompile(`^[^@]+@([^@]+)\.[^.@]*$`)

//  fullDomainRe is a regex used to obtain the full domain (with top level
//  domain) out of an email string.
//  e.g. a@managedchrome.com -> [a@managedchrome.com managedchrome.com] and
//  ex2@domainp1.domainp2.com -> [ex2@domainp1.domainp2.com domainp1.domainp2.com]
var fullDomainRe = regexp.MustCompile(`^[^@]+@([^@]+)$`)

// userDomain will return the "domain" section (without top level domain) of
// user.
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

// fullUserDomain will return the full "domain" (including top level domain) of
// user.
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

// findEnrollmentTargets returns the Gaia WebView targets, which are used to
// help enrollment on the device.
// Returns nil if none are found.
func findEnrollmentTargets(ctx context.Context, sess *driver.Session, userDomain string) ([]*driver.Target, error) {
	isGAIAWebView := func(t *driver.Target) bool {
		return t.Type == "webview" && isGAIASignInURL(t.URL)
	}

	ts, err := sess.FindTargets(ctx, isGAIAWebView)
	if err != nil {
		return nil, err
	}

	// It's common for multiple targets to be returned.
	// We want to run the command specifically on the "apps" target.
	var targets []*driver.Target
	for _, t := range ts {
		u, err := url.Parse(t.URL)
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
				targets = append(targets, t)
			}
		}
	}

	return targets, nil
}

// matchTargetDomains returns a function that matches only the GAIA WebView
// target for post-enrollment enterprise account sign in.
// Used by test automation to distinguish between multiple GAIA webview
// targets in the OOBE after enterprise enrollment.
func matchTargetDomains(ctx context.Context, sess *driver.Session, fullDomain, userDomain string) cdputil.TargetMatcher {
	return func(t *driver.Target) bool {
		// First, check if the webview is a valid GAIA sign in target.
		if !MatchSignInGAIAWebView(ctx, sess)(t) {
			return false
		}

		// Next, check that the webview url has the right set of query parameters.
		u, err := url.Parse(t.URL)
		if err != nil {
			return false
		}

		q := u.Query()
		clientID := q.Get("client_id")
		managedDomain := q.Get("manageddomain")
		flowName := q.Get("flowName")

		if !strings.Contains(clientID, "apps.googleusercontent.com") ||
			!strings.Contains(managedDomain, userDomain) ||
			!strings.Contains(flowName, "SetupChromeOs") {
			return false
		}

		// Finally, check that the webview has a login banner with the enrolled
		// enterprise's full domain.
		loginBanner := fmt.Sprintf(`document.querySelectorAll('span[title=%q]').length;`, fullDomain)

		conn, err := sess.NewConnForTarget(ctx, driver.MatchTargetID(t.TargetID))
		if err != nil {
			return false
		}
		defer conn.Close()
		content := -1
		if err := conn.Eval(ctx, loginBanner, &content); err != nil {
			return false
		}
		return content == 1
	}
}

// waitForEnrollmentLoginScreen waits for the Enrollment screen to complete
// and the Enrollment login screen to appear. If the login screen does not
// appear testing.Poll times out.
func waitForEnrollmentLoginScreen(ctx context.Context, cfg *config.Config, sess *driver.Session) error {
	testing.ContextLog(ctx, "Waiting for enrollment login screen")

	// Wait for the enrollment OOBE page to disappear.
	if err := waitForPageWithPrefixToBeDismissed(ctx, sess, "chrome://oobe/oobe"); err != nil {
		return errors.Wrap(err, "enrollment OOBE screen did not disappear")
	}

	// Wait for the signin OOBE page to appear.
	oobeConn, err := WaitForOOBEConnectionWithPrefix(ctx, sess, "chrome://oobe/gaia-signin")
	if err != nil {
		return errors.Wrap(err, "could not find OOBE connection for gaia sign in")
	}
	defer oobeConn.Close()

	js := "OobeAPI.screens.GaiaScreen.isReadyForTesting()"
	if err := oobeConn.WaitForExprWithTimeout(ctx, js, 10*time.Second); err != nil {
		return errors.Wrap(err, "the signin screen is not ready")
	}

	user := cfg.EnrollmentCreds().User

	fullDomain, err := fullUserDomain(user)
	if err != nil {
		return errors.Wrap(err, "no valid full user domain found")
	}

	userDomain, err := userDomain(user)
	if err != nil {
		return errors.Wrap(err, "no valid user domain found")
	}

	if _, err := waitForSingleGAIAWebView(waitForWebViewOptions{
		Context:        ctx,
		Session:        sess,
		TargetMatcher:  matchTargetDomains(ctx, sess, fullDomain, userDomain),
		PollingOptions: &testing.PollOptions{Timeout: 45 * time.Second},
	}); err != nil {
		msg := "failed to find the enterprise sign-in GAIA webview"
		return errors.Wrap(sess.Watcher().ReplaceErr(err), msg)
	}
	return nil
}

// performFakeEnrollment performs enterprise enrollment with a fake, local
// device management server and wait for it to complete.
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

// performGAIAEnrollment enrolls the test device using the OOBE screen.
func performGAIAEnrollment(ctx context.Context, cfg *config.Config, sess *driver.Session) error {
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
		msg := "no Internet connectivity, cannot perform GAIA enrollment"
		return errors.Wrap(err, msg)
	}

	if err := conn.Call(ctx, nil, "Oobe.skipToLoginForTesting"); err != nil {
		return err
	}

	js := "OobeAPI.screens.GaiaScreen.isReadyForTesting()"
	if err := conn.WaitForExpr(ctx, js); err != nil {
		msg := "failed to wait for the OOBE Gaia sign in screen"
		return errors.Wrap(err, msg)
	}

	js = "Oobe.switchToEnterpriseEnrollmentForTesting"
	if err := conn.Call(ctx, nil, js); err != nil {
		return err
	}

	if err := performGAIAEnrollmentSignIn(ctx, conn, creds, sess); err != nil {
		return err
	}

	if err := waitForEnrollmentLoginScreen(ctx, cfg, sess); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "could not enroll")
	}

	return nil
}

// performGAIAEnrollmentSignIn performs GAIA enrollment using the given
// credentials.
// Uses maxGAIAEnterpriseEnrollmentRetries as the retry count.
// Uses enrollmentTimeout as the timeout limit.
func performGAIAEnrollmentSignIn(ctx context.Context, oobeConn *driver.Conn, creds config.Creds, sess *driver.Session) error {
	retries := maxGAIAEnterpriseEnrollmentRetries
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := submitGAIAEnrollmentSignIn(ctx, oobeConn, creds, sess); err != nil {
			return testing.PollBreak(err)
		}

		js := "OobeAPI.screens.EnterpriseEnrollmentScreen.successStep.isReadyForTesting()"
		if err := oobeConn.WaitForExprFailOnErr(ctx, js); err == nil {
			js = "OobeAPI.screens.EnterpriseEnrollmentScreen.successStep.clickNext()"
			if err := oobeConn.Eval(ctx, js, nil); err != nil {
				msg := "failed to click the enrollment done button"
				return testing.PollBreak(errors.Wrap(err, msg))
			}
			return nil
		}

		// Sometimes enrollment may fail due to one-off issues with the device
		// management server.
		// Check if enrollment maybe retried.
		var isOnErrorStep bool
		js = "OobeAPI.screens.EnterpriseEnrollmentScreen.errorStep.isReadyForTesting()"
		if err := oobeConn.Eval(ctx, js, &isOnErrorStep); err != nil {
			msg := "failed to check enrollment step"
			return testing.PollBreak(errors.Wrap(err, msg))
		}

		if !isOnErrorStep {
			msg := "unexpected step after enrollment signin failure"
			return testing.PollBreak(errors.New(msg))
		}

		var canRetry bool
		js = "OobeAPI.screens.EnterpriseEnrollmentScreen.errorStep.canRetryEnrollment()"
		if err := oobeConn.Eval(ctx, js, &canRetry); err != nil {
			msg := "failed to check enrollment step"
			return testing.PollBreak(errors.Wrap(err, msg))
		}

		if !canRetry {
			var enrollmentErrorMsg string
			js = "OobeAPI.screens.EnterpriseEnrollmentScreen.errorStep.getErrorMsg()"
			if err := oobeConn.Eval(ctx, js, &enrollmentErrorMsg); err != nil {
				msg := "failed to get unretriable enrollment error msg"
				return testing.PollBreak(errors.Wrap(err, msg))
			}
			msg := "enrollment hit an unrecoverable error: %v"
			return testing.PollBreak(errors.Errorf(msg, enrollmentErrorMsg))
		}

		retries--
		if retries <= 0 {
			return testing.PollBreak(errors.New("exhausted retries"))
		}

		js = "OobeAPI.screens.EnterpriseEnrollmentScreen.errorStep.clickRetryButton()"
		if err := oobeConn.Eval(ctx, js, nil); err != nil {
			msg := "failed to click the retry button"
			return testing.PollBreak(errors.Wrap(err, msg))
		}

		return errors.New("temporary enrollment error")
	}, &testing.PollOptions{Timeout: enrollmentTimeout, Interval: time.Millisecond})
}

// isEnrollmentWebView checks if the WebView in the specified driver.Target
// is the enterprise enrollment WebView. Used by automation to distinguish
// multiple WebView targets in ChromeOS's OOBE.
func isEnrollmentWebView(t *driver.Target) bool {
	if t.Type != "webview" || !isGAIASignInURL(t.URL) {
		return false
	}

	u, err := url.Parse(t.URL)
	if err != nil {
		return false
	}

	q := u.Query()
	flow := q.Get("flow")

	if flow == "enterprise" {
		return true
	}

	return false
}

// submitGAIAEnrollmentSignIn submits the enrollment GAIA credentials
// (user email + password) through the GAIA webview on the OOBE enrollment page.
func submitGAIAEnrollmentSignIn(ctx context.Context, oobeConn *driver.Conn, creds config.Creds, sess *driver.Session) error {
	js := "OobeAPI.screens.EnterpriseEnrollmentScreen.signInStep.isReadyForTesting()"
	if err := oobeConn.WaitForExprFailOnErr(ctx, js); err != nil {
		msg := "failed to wait for the OOBE enterprise enrollment signin screen to be ready"
		return errors.Wrap(err, msg)
	}

	target, err := waitForSingleGAIAWebView(waitForWebViewOptions{
		Context:        ctx,
		Session:        sess,
		TargetMatcher:  isEnrollmentWebView,
		PollingOptions: pollOpts,
	})
	if err != nil {
		msg := "failed to find GAIA webview"
		return errors.Wrap(sess.Watcher().ReplaceErr(err), msg)
	}

	gaiaConn, err := sess.NewConnForTarget(ctx, driver.MatchTargetID(target.TargetID))
	if err != nil {
		msg := "failed to connect to GAIA webview"
		return errors.Wrap(sess.Watcher().ReplaceErr(err), msg)
	}
	defer gaiaConn.Close()

	if err := insertGAIAField(ctx, gaiaConn, "#identifierId", creds.User); err != nil {
		return errors.Wrap(err, "failed to fill username field")
	}

	js = "Oobe.clickGaiaPrimaryButtonForTesting"
	if err := oobeConn.Call(ctx, nil, js); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	js = "OobeAPI.screens.EnterpriseEnrollmentScreen.signInStep.isReadyForTesting()"
	if err := oobeConn.WaitForExprFailOnErr(ctx, js); err != nil {
		msg := "failed to wait for the OOBE enterprise enrollment signin screen to be ready"
		return errors.Wrap(err, msg)
	}

	if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", creds.Pass); err != nil {
		return errors.Wrap(err, "failed to fill in password field")
	}

	js = "Oobe.clickGaiaPrimaryButtonForTesting"
	if err := oobeConn.Call(ctx, nil, js); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	js = "!OobeAPI.screens.EnterpriseEnrollmentScreen.isEnrollmentInProgress()"
	testing.ContextLog(ctx, "Wait for enrollment to complete")
	if err := oobeConn.WaitForExprFailOnErr(ctx, js); err != nil {
		return errors.Wrap(err, "failed to wait for enrollment to complete")
	}

	return nil
}
