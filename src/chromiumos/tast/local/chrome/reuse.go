// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/internal/extension"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/testing"
)

// tryReuseSession checks if the exiting chrome session can be reuse, and returns a
// Chrome instance if reuse criteria is met.
func tryReuseSession(ctx context.Context, cfg *config.Config) (cr *Chrome, retErr error) {
	testing.ContextLog(ctx, "Trying to reuse existing chrome session")

	if !cfg.TryReuseSession {
		return nil, errors.New("TryReuseSession option is not set")
	}

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

	if err := compareUserLogin(ctx, sess, cfg.User); err != nil {
		return nil, err
	}

	return &Chrome{
		cfg:          *cfg,
		exts:         nil,
		agg:          agg,
		sess:         sess,
		loginPending: cfg.DeferLogin,
	}, nil
}

// compareExtensions checks if the new session extensions match the ones of exsting session.
func compareExtensions(cfg *config.Config) error {
	// Get existing extension checksums.
	extsDir := filepath.Join(persistentDir, "extensions")
	checksums, err := extension.Checksums(extsDir)
	if err != nil {
		return errors.Wrap(err, "failed to get checksums of existing extensions")
	}

	// Prepare extensions for new session in a temporary dir.
	tempDir, err := ioutil.TempDir("/tmp", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	_, err = extension.PrepareExtensions(filepath.Join(tempDir, "extensions"), cfg)
	if err != nil {
		return err
	}
	newChecksums, err := extension.Checksums(tempDir)
	if err != nil {
		return errors.Wrap(err, "failed to get checksums of new extensions")
	}

	// Make sure all new extensions exist in existing extensions.
	if len(newChecksums) > len(checksums) {
		return errors.New("extensions don't match")
	}
	contains := func(slc []string, ele string) bool {
		for _, e := range slc {
			if e == ele {
				return true
			}
		}
		return false
	}
	for _, ne := range newChecksums {
		if !contains(checksums, ne) {
			return errors.New("new extension doesn't exist")
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
	var user struct {
		Email string
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

	if err := conn.Eval(ctx, `tast.promisify(chrome.usersPrivate.getCurrentUser)()`, &user); err != nil {
		return errors.Wrap(err, "failed to run javascript to get current user")
	}
	if user.Email != email {
		return errors.New("existing session is logged in with a different user")
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
	if err := conn.Eval(ctx, extension.TastChromeOptioionsJSVar, &existingChromeOptions); err != nil {
		return errors.Wrapf(err, "failed to evaluate %s", extension.TastChromeOptioionsJSVar)
	}

	existingCfg := &config.Config{}
	if err := json.Unmarshal([]byte(existingChromeOptions), existingCfg); err != nil {
		return errors.Wrap(err, "failed to unmarshal existing config")
	}

	if !cfg.IsSessionReusable(existingCfg) {
		return errors.New("configurations are not compatible for reuse")
	}

	return nil
}
