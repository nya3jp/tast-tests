// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWGetSet,
		Desc:     "Test IW getter and setter functions",
		Contacts: []string{"deanliao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func IWGetSet(ctx context.Context, s *testing.State) {
	const iface = "wlan0"
	iwr := iw.NewRunner()
	res, err := iwr.GetRegulatoryDomain(ctx)
	if err != nil {
		s.Fatal("GetRegulatoryDomain failed: ", err)
	}
	s.Log("Regulatory Domain: ", res)
	// TODO: Flesh out the test and add unit tests to include more of the getters/setters. Tests
	// can't really be made for them right now because they require a link to
	// an AP.
}
