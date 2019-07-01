// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	androidDataDirPath = "/opt/google/containers/android/rootfs/android-data/data"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AuthPerf,
		Desc:         "Measure auth times in ARC++",
		Contacts:     []string{"niwa@chromium.org", "khmel@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"android", "chrome"},
		Timeout:      2 * time.Minute,
	})
}

func AuthPerf(ctx context.Context, s *testing.State) {
	const (
		username      = "crosauthperf@gmail.com"
		password      = "54JUxo=3Lf1zLMVE"
		gaiaID        = "1234"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(),
	                      chrome.GAIALogin(),
	                      chrome.Auth(username, password, gaiaID),
	                      chrome.ExtraArgs("--arc-force-show-optin-ui"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

    PerformArcboot(ctx, s, cr)
}

func PerformArcboot(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	EnablePlayStore(ctx, s, cr, false)
    WaitForAndroidDataRemoved(ctx, s)
}

func EnablePlayStore(ctx context.Context, s *testing.State, cr *chrome.Chrome, enabled bool) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.setPlayStoreEnabled(%s, () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, strconv.FormatBool(enabled))

	if err = tconn.EvalPromise(ctx, expr, nil); err != nil {
		s.Fatal("Running autotestPrivate.setPlayStoreEnabled failed: ", err)
	}
}

func WaitForAndroidDataRemoved(ctx context.Context, s *testing.State) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(androidDataDirPath); os.IsNotExist(err) {
			return nil
		} else {
            return errors.Wrap(err, "Android data still exists")
		}
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil { // this should be 10 second
		s.Fatal("Failed to wait for Android data folder is removed: ", err)
	}
}
