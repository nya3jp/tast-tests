// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shillscript"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShillInitScriptsStartLoggedin,
		Desc:         "Test that shill init scripts perform as expected",
		Contacts:     []string{"arowa@google.com", "cros-networking@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func ShillInitScriptsStartLoggedin(ctx context.Context, s *testing.State) {
	if err := shillscript.RunTest(ctx, testStartLoggedIn, false); err != nil {
		s.Fatal("Failed running testStartLoggedIn: ", err)
	}
}

// testStartLoggedIn tests starting up shill while user is already logged in.
func testStartLoggedIn(ctx context.Context, env *shillscript.TestEnv) error {
	cr, err := chrome.New(ctx)
	if err != nil {
		return errors.Wrap(err, "Chrome failed to log in")
	}
	defer cr.Close(ctx)

	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	return nil
}
