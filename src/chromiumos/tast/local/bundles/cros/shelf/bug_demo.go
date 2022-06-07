// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BugDemo,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests the basic features of the overflow shelf",
		Contacts: []string{
			"andrewxu@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Fixture:      "install100Apps",
	})
}

// BugDemo ...
func BugDemo(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome
	var err error
	opts := s.FixtValue().([]chrome.Option)

	// When commenting this line, the code works.
	opts = append(opts, chrome.ExtraArgs("--force-ui-direction=rtl"))

	cr, err = chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start chrome with rtl: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// When enabling --force-ui-direction=rtl, it throws the error that
	// "Failed to enter overflow: got 0 apps, want at least 10 apps". Actually,
	// if you print the app names in EnterShelfOverflow(), you will find that
	// fake apps exist. But somehow the prefix checks always return false, even
	// for the app names like "fake app 0". In addition, if you print the first
	// four characters of a app name (by app_name[0:4]), it will complain about
	// the illegal utf-8 data.
	if err := ash.EnterShelfOverflow(ctx, tconn); err != nil {
		s.Fatal("Failed to enter overflow: ", err)
	}

}
