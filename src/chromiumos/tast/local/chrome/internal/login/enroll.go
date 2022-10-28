// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/internal/cdputil"
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

// gaiaEnterpriseEnrollmentTimeout is the maximum amount of time to wait for enrollment to
// succeed.
const gaiaEnterpriseEnrollmentTimeout = 3 * time.Minute

// domainRe is a regex used to obtain the domain (without top level domain)
// out of an email string.
// e.g. a@managedchrome.com -> [a@managedchrome.com managedchrome] and
// ex2@domainp1.domainp2.com -> [ex2@domainp1.domainp2.com domainp1.domainp2]
var domainRe = regexp.MustCompile(`^[^@]+@([^@]+)\.[^.@]*$`)

// fullDomainRe is a regex used to obtain the full domain (with top level
// domain) out of an email string.
// e.g. a@managedchrome.com -> [a@managedchrome.com managedchrome.com] and
// ex2@domainp1.domainp2.com -> [ex2@domainp1.domainp2.com domainp1.domainp2.com]
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

// findEnrollmentTargets returns the Gaia WebView targets, that are used to
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
		deviceManager := q.Get("devicemanager")
		flowName := q.Get("flowName")

		if !strings.Contains(clientID, "apps.googleusercontent.com") ||
			!strings.Contains(deviceManager, userDomain) ||
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

	// Login window may not be shown yet if for example managed guest session is
	// enabled.
	if err := oobeConn.Eval(ctx, "Oobe.showAddUserForTesting()", nil); err != nil {
		return err
	}

	if err := oobeConn.WaitForExprWithTimeout(ctx, "OobeAPI.screens.GaiaScreen.isReadyForTesting()", 60*time.Second); err != nil {
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

	mt := matchTargetDomains(ctx, sess, fullDomain, userDomain)
	if _, err := waitForSingleGAIAWebView(ctx, sess, mt, 45*time.Second); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to find the enterprise sign-in GAIA webview")
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
		return errors.Wrap(err, "no Internet connectivity, cannot perform GAIA enrollment")
	}

	if err := conn.Call(ctx, nil, "Oobe.switchToEnterpriseEnrollmentForTesting"); err != nil {
		return err
	}

	if err := performGAIAEnrollmentSignIn(ctx, conn, creds, sess); err != nil {
		return err
	}

	if cfg.LoginMode() == config.NoLogin {
		return nil
	}

	if err := waitForEnrollmentLoginScreen(ctx, cfg, sess); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "could not enroll")
	}

	return nil
}

// performGAIAZTEEnrollment enrolls the test device using the OOBE screen.
func performGAIAZTEEnrollment(ctx context.Context, cfg *config.Config, sess *driver.Session) error {
	ctx, st := timing.Start(ctx, "zteenroll")
	defer st.End()

	conn, err := WaitForOOBEConnection(ctx, sess)
	if err != nil {
		return errors.Wrap(err, "could not find OOBE connection")
	}

	if err := conn.WaitForExpr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		return errors.Wrap(err, "failed to wait for the OOBE Welcome Screen")
	}

	if err := conn.Eval(ctx, "OobeAPI.screens.WelcomeScreen.clickNext()", nil); err != nil {
		return errors.Wrap(err, "failed to click on the Next button on the OOBE Welcome Screen")
	}

	if err := conn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.successStep.isReadyForTesting()"); err != nil {
		return errors.Wrap(err, "failed to wait for the OOBE enterprise enrollment signin screen to be ready")
	}

	defer conn.Close()
	return nil
}

// performGAIAEnrollmentSignIn performs GAIA enrollment using the given
// credentials.
// Uses maxGAIAEnterpriseEnrollmentRetries as the retry count.
// Uses gaiaEnterpriseEnrollmentTimeout as the timeout limit.
func performGAIAEnrollmentSignIn(ctx context.Context, oobeConn *driver.Conn, creds config.Creds, sess *driver.Session) error {
	retries := maxGAIAEnterpriseEnrollmentRetries
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := submitGAIAEnrollmentSignIn(ctx, oobeConn, creds, sess); err != nil {
			return testing.PollBreak(err)
		}

		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.successStep.isReadyForTesting()"); err == nil {
			if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.successStep.clickNext()", nil); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to click the enrollment done button"))
			}
			return nil
		}

		// Sometimes enrollment may fail due to one-off issues with the device
		// management server.
		// Check if enrollment maybe retried.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			var isOnErrorStep bool
			if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.errorStep.isReadyForTesting()", &isOnErrorStep); err != nil {
				return errors.Wrap(err, "failed to check if error step is ready")
			}

			if !isOnErrorStep {
				return errors.New("unexpected step after enrollment signin failure")
			}

			var canRetry bool
			if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.errorStep.canRetryEnrollment()", &canRetry); err != nil {
				return errors.Wrap(err, "failed to check if retry can be attempted")
			}

			if !canRetry {
				var enrollmentErrorMsg string
				if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.errorStep.getErrorMsg()", &enrollmentErrorMsg); err != nil {
					return errors.Wrap(err, "failed to get unretriable enrollment error msg")
				}
				return errors.Errorf("enrollment hit an unrecoverable error: %v", enrollmentErrorMsg)
			}

			return nil
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check if enrollment can be retried"))
		}

		retries--
		if retries <= 0 {
			return testing.PollBreak(errors.New("exhausted retries"))
		}

		if err := oobeConn.Eval(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.errorStep.clickRetryButton()", nil); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to click the retry button"))
		}

		return errors.New("temporary enrollment error")
	}, &testing.PollOptions{Timeout: gaiaEnterpriseEnrollmentTimeout, Interval: time.Millisecond})
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
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.signInStep.isReadyForTesting()"); err != nil {
		return errors.Wrap(err, "failed to wait for the OOBE enterprise enrollment signin screen to be ready")
	}

	target, err := waitForSingleGAIAWebView(ctx, sess, isEnrollmentWebView, pollOpts.Timeout)
	if err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to find GAIA webview")
	}

	gaiaConn, err := sess.NewConnForTarget(ctx, driver.MatchTargetID(target.TargetID))
	if err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "failed to connect to GAIA webview")
	}
	defer gaiaConn.Close()

	if err := insertGAIAField(ctx, gaiaConn, "#identifierId", creds.User); err != nil {
		return errors.Wrap(err, "failed to fill username field")
	}

	if err := oobeConn.Call(ctx, nil, "Oobe.clickGaiaPrimaryButtonForTesting"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EnterpriseEnrollmentScreen.signInStep.isReadyForTesting()"); err != nil {
		return errors.Wrap(err, "failed to wait for the OOBE enterprise enrollment signin screen to be ready")
	}

	if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", creds.Pass); err != nil {
		return errors.Wrap(err, "failed to fill in password field")
	}

	if err := oobeConn.Call(ctx, nil, "Oobe.clickGaiaPrimaryButtonForTesting"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	testing.ContextLog(ctx, "Wait for enrollment to complete")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "!OobeAPI.screens.EnterpriseEnrollmentScreen.isEnrollmentInProgress()"); err != nil {
		return errors.Wrap(err, "failed to wait for enrollment to complete")
	}

	return nil
}
