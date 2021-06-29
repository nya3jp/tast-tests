// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/internal/extension"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/logsaver"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// tryReuseSession checks if the exiting chrome session can be reuse, and returns a
// Chrome instance if reuse criteria is met.
func tryReuseSession(ctx context.Context, cfg *config.Config) (cr *Chrome, retErr error) {
	ctx, st := timing.Start(ctx, "try_reuse_session")
	defer st.End()

	testing.ContextLog(ctx, "Trying to reuse existing chrome session")

	agg := jslog.NewAggregator()
	defer func() {
		if retErr != nil {
			agg.Close()
		}
	}()
	sess, err := driver.NewSession(ctx, cdputil.DebuggingPortPath, cdputil.NoWaitPort, agg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			sess.Close(ctx)
		}
	}()

	if err := compareExtensions(cfg); err != nil {
		return nil, err
	}

	if err := compareConfig(ctx, sess, cfg); err != nil {
		return nil, err
	}

	if err := compareUserLogin(ctx, sess, cfg.Creds().User); err != nil {
		return nil, err
	}

	logFilename, err := CurrentLogFile()
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Log file name: %s", logFilename)
	// When reusing the session, lines already in the log file should be unrelated
	// to the test itself. Thus use logsaver.NewMarker to exclude already existing
	// log lines.
	logMarker, err := logsaver.NewMarker(logFilename)
	if err != nil {
		return nil, err
	}

	return &Chrome{
		cfg:          *cfg,
		agg:          agg,
		sess:         sess,
		logFilename:  logFilename,
		logMarker:    logMarker,
		loginPending: cfg.DeferLogin(),
	}, nil
}

// compareExtensions checks if the new session extensions match the ones of the existing session.
func compareExtensions(cfg *config.Config) error {
	// Get existing extension checksums.
	checksums, err := extension.Checksums(extensionsDir)
	if err != nil {
		return errors.Wrap(err, "failed to get checksums of existing extensions")
	}

	// Prepare extensions for new session in a temporary dir.
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	guestModeLogin := extension.GuestModeDisabled
	if cfg.LoginMode() == config.GuestLogin {
		guestModeLogin = extension.GuestModeEnabled
	}
	// PrepareExtensions() expects a non-existent extension dir.
	tempExtDir := filepath.Join(tempDir, "extensions")
	_, err = extension.PrepareExtensions(tempExtDir, cfg, guestModeLogin)
	if err != nil {
		return err
	}
	newChecksums, err := extension.Checksums(tempExtDir)
	if err != nil {
		return errors.Wrap(err, "failed to get checksums of new extensions")
	}

	// Make sure all new extensions exist in existing extensions by comparing checksums.
	contains := func(slc []string, ele string) bool {
		// Check if a slice contains a given element.
		for _, e := range slc {
			if e == ele {
				return true
			}
		}
		return false
	}
	for _, ne := range newChecksums {
		if !contains(checksums, ne) {
			return errors.New("required extension doesn't exist in the existing session")
		}
	}
	return nil
}

// compareUserLogin checks if the configured user has logged in.
func compareUserLogin(ctx context.Context, sess *driver.Session, email string) error {
	conn, err := sess.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to establish Chrome connection")
	}

	var status struct {
		IsLoggedIn     bool
		IsScreenLocked bool
	}

	if err := conn.Eval(ctx, `tast.promisify(chrome.usersPrivate.getLoginStatus)()`, &status); err != nil {
		return errors.Wrap(err, "failed to run javascript to get login status")
	}
	if !status.IsLoggedIn {
		return errors.New("user has not logged in")
	}
	if status.IsScreenLocked {
		return errors.New("screen is locked")
	}

	return nil
}

// compareConfig compares the configuration between new and existing Chrome sessions.
func compareConfig(ctx context.Context, sess *driver.Session, cfg *config.Config) error {
	conn, err := sess.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to establish Chrome connection")
	}

	var existingChromeOptions string
	if err := conn.Eval(ctx, extension.TastChromeOptionsJSVar, &existingChromeOptions); err != nil {
		return errors.Wrapf(err, "failed to evaluate %s", extension.TastChromeOptionsJSVar)
	}

	existingCfg, err := config.Unmarshal([]byte(existingChromeOptions))
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal existing config data")
	}

	return existingCfg.VerifySessionReuse(cfg)
}
