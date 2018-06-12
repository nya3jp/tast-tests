// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"fmt"
	"os"

	"chromiumos/tast/testing"
)

const (
	androidImageDir = "/opt/google/containers/android"
)

// isARCAvailable returns an error if the directory containing the Android system
// image is missing or can't be read.
func isARCAvailable() error {
	if _, err := os.Stat(androidImageDir); os.IsNotExist(err) {
		return fmt.Errorf("missing Android image dir %v", androidImageDir)
	} else if err != nil {
		return err
	}
	return nil
}

// enableARC enables ARC on the current session.
func enableARC(ctx context.Context, c *Chrome) error {
	testing.ContextLog(ctx, "Enabling ARC")
	conn, err := c.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	// TODO(derat): Consider adding more functionality (e.g. checking managed state)
	// from enable_play_store() in Autotest's client/common_lib/cros/arc_util.py.
	return conn.Exec(ctx, "chrome.autotestPrivate.setPlayStoreEnabled(true, function(enabled) {});")
}
