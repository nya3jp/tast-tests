// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IwScan,
		Desc:     "Verifies `iw` Timed Scan executes and is parsed properly",
		Contacts: []string{"billyzhao@gogle.com"},
		Attr:     []string{"informational"},
	})
}

func IwScan(ctx context.Context, s *testing.State) {
	iface := "wlan0"
	frequencies := []int{}
	ssids := []string{}
	_, err := iw.TimedScan(ctx, s, iface, frequencies, ssids)
	if err != nil {
		s.Fatal(err.Error())
	}
	return
}
