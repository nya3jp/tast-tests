// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mafredri/cdp/protocol/target"

	"go.chromium.org/chromiumos/tast-tests/local/chrome/internal/cdputil"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/internal/config"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/internal/driver"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/internal/extension"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/jslog"
)

// DeprecatedNewConn starts a new session using sm for communicating with the supplied target.
// pageURL is only used when logging JavaScript console messages via lm.
//
// DEPRECATED: Do not call this function. It's available only for compatibility
// with old code.
func DeprecatedNewConn(ctx context.Context, s *cdputil.Session, id target.ID, la *jslog.Aggregator, pageURL string, chromeErr func(error) error) (c *Conn, retErr error) {
	return driver.NewConn(ctx, s, id, la, pageURL, chromeErr)
}

// DeprecatedPrepareExtensions prepares test extensions and returns extension
// directory paths.
//
// DEPRECATED: Do not call this function. It's available only for compatibility
// with old code.
func DeprecatedPrepareExtensions() (extDirs []string, err error) {
	dir, err := ioutil.TempDir("", "tast_test_api_extension.")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0755); err != nil {
		return nil, err
	}

	cfg, err := config.NewConfig(nil)
	if err != nil {
		return nil, err
	}
	exts, err := extension.PrepareExtensions(filepath.Join(dir, "extensions"), cfg, extension.GuestModeDisabled)
	if err != nil {
		return nil, err
	}
	return exts.DeprecatedDirs(), nil
}
