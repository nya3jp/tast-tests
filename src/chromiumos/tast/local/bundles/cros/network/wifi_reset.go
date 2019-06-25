// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/ping"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"context"
	"fmt"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WifiReset,
		Desc:     "Ensures that WiFi chip can recover from suspend resume properly",
		Contacts: []string{"billyzhao@google.com"},
	})
}
func WifiReset(ctx context.Context, s *testing.State) {
	_, err := ping.SimplePing(ctx, s, "8.8.8.8")
	if err != nil {
		s.Fatal(errors.Wrap(err, "First SimplePing failed.").Error())
	}
	s.Log("First Ping succeeded")
	_, err = testexec.CommandContext(ctx, "suspend_stress_test", "-c", "1").Output()
	if err != nil {
		s.Fatal(errors.Wrap(err, "Reset failed.").Error())
	}
	s.Log("Suspend resume succeeded.")
	res, err := ping.SimplePing(ctx, s, "8.8.8.8")
	if err != nil {
		s.Fatal(errors.Wrap(err, "Second SimplePing failed.").Error())
	}
	s.Log(fmt.Sprintf("Second Simple ping succeeded, res: %v", res))

}
