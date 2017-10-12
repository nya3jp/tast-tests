// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"os/exec"

	"chromiumos/tast/common/testing"
)

// enablePlayStore enables the Google Play Store, needed by ARC to boot Android.
func enablePlayStore(ctx context.Context, c *Chrome) error {
	testing.ContextLog(ctx, "Enabling Play Store")
	conn, err := c.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	// TODO(derat): Consider adding more functionality (e.g. checking managed state)
	// from enable_play_store() in Autotest's client/common_lib/cros/arc_util.py.
	return conn.Exec(ctx, "chrome.autotestPrivate.setPlayStoreEnabled(true, function(enabled) {});")
}

// waitForAndroidBooted waits for the Android container to report that it's finished booting.
func waitForAndroidBooted(ctx context.Context) error {
	testing.ContextLog(ctx, "Waiting for Android to boot (per \"getprop sys.boot_completed\")")

	// android-sh introduces a lot of overhead, so poll within the android-sh command.
	// Rerun android-sh every ten seconds to ensure we don't spin indefinitely.
	ch := make(chan error, 1)
	go func() {
		f := func() bool {
			loop := "for i in $(seq 0 99); do " +
				"getprop sys.boot_completed | grep -q 1 && exit 0; sleep 0.1; done; exit 1"
			cmd := exec.Command("android-sh", "-c", loop)
			return cmd.Run() == nil
		}
		ch <- poll(ctx, f)
	}()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
