// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWGetSet,
		Desc:     "Test IW getter and setter functions",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func IWGetSet(ctx context.Context, s *testing.State) {
	const iface = "wlan0"
	res, err := iw.GetRegulatoryDomain(ctx)
	if err != nil {
		s.Fatal("GetRegulatoryDomain failed: ", err)
	}
	s.Log("Regulatory Domain: ", res)
}
