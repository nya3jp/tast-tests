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

		if clientID != "" && managedDomain != "" {
			if strings.Contains(clientID, "apps.googleusercontent.com") &&
				strings.Contains(managedDomain, userDomain) {
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
				// Enrollment will only exceed if the eval below succeeds.
				continue
			}
			defer webViewConn.Close()
			content := -1
			if err := webViewConn.Eval(ctx, loginBanner, &content); err != nil {
				return err
			}
			// Found the login screen.
			if content == 1 {
				return nil
			}
		}
		return errors.New("Enterprise Enrollment login screen not found")
	}, &testing.PollOptions{Timeout: 45 * time.Second}); err != nil {
		return err
	}

	return nil
}

// performEnrollment will perform enrollment and wait for it to complete.
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
	if err := conn.Call(ctx, nil, "Oobe.loginForTesting", creds.User, creds.Pass, creds.GAIAID, true); err != nil {
		return errors.Wrap(err, "failed to trigger enrollment")
	}

	if err := waitForEnrollmentLoginScreen(ctx, cfg, sess); err != nil {
		return errors.Wrap(sess.Watcher().ReplaceErr(err), "could not enroll")
	}

	return nil
}
