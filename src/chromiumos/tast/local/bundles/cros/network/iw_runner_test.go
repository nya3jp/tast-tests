// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/testing"
	"context"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IwRunnerTest,
		Desc:     "Test iwRunnerFuncs",
		Contacts: []string{"billyzhao@google.com"},
	})
}

func IwRunnerTest(ctx context.Context, s *testing.State) {
	iface := "wlan0"
	frequencies := []int{1, 2, 3}
	ssids := []string{"hi"}
	iwr := iw.NewIwRunner(s, ctx)
	iwr.TimedScan(iface, frequencies, ssids)
	return
}
