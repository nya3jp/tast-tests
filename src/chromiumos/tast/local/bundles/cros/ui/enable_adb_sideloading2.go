// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"errors"
	"fmt"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnableAdbSideloading2,
		Desc:         "....Checks that Chrome supports login",
		Contacts:     []string{"chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func initiateAdbSideloadingFlow(ctx context.Context, s *testing.State, tconn *chrome.Conn) error {
	s.Log("Setting preference to start enable-sideloading flow")
	if err := tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.autotestPrivate.setWhitelistedPref(
		'EnableAdbSideloadingRequested', true, () => {
			if (chrome.runtime.lastError) {
				reject(chrome.runtime.lastError.message);
			} else {
				resolve();
			};
		}
	);
})
`, nil); err != nil {
		return err
	}
	return nil
}

func getAdbSideloading(ctx context.Context) (bool, error) {
	cmd := testexec.CommandContext(ctx, "bootlockboxtool", "--action=read", "--key=arc_sideloading_allowed")
	data, err := cmd.Output()
	if err != nil {
		return false, err
	}
	enabled := string(data)
	if enabled == "0" {
		return false, nil
	} else if enabled == "1" {
		return true, nil
	} else {
		return false, errors.New(fmt.Sprintf("unexpected value in bootlockbox: %s", enabled))
	}
}

func EnableAdbSideloading2(ctx context.Context, s *testing.State) {
	// TODO filter devices by support
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	//defer tconn.Close()

	if err := initiateAdbSideloadingFlow(ctx, s, tconn); err != nil {
		s.Fatal("Failed to initiate flow to enable adb sideloading", err)
	}

	// Actually reboot
	if err = upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Chrome logout failed: ", err)
	}
	/////

	if err := tconn.EvalPromise(ctx, `
	$('adb-sideloading').shadowRoot.getElementById('enable-adb-sideloading-ok-button').shadowRoot.getElementById('textButton').click()
`, nil); err != nil {
		s.Fatal("Orz", err)
	}
	enabled, err := getAdbSideloading(ctx)
	if err != nil {
		s.Fatal("Failed to read from bootlockbox")
	}

	if !enabled {
		s.Fatal("Failed to enable adb sideloading")
	}

	// TODO reset
}
