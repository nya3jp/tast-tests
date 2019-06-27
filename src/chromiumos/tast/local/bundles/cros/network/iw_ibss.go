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
		Func:     IWIBSS,
		Desc:     "Verifies `iw` IBSS behavior",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func IWIBSS(ctx context.Context, s *testing.State) {
	phy := "phy0"
	iface := "ah0"
	defer func() {
		s.Log("cleanup")
		iw.IBSSLeave(ctx, iface)
		iw.RemoveInterface(ctx, iface)
	}()
	err := iw.AddInterface(ctx, phy, iface, "ibss")
	if err != nil {
		s.Fatal("AddInterface failed: ", err)
	}
	err = iw.IBSSJoin(ctx, iface, "Test", 2412)
	if err != nil {
		s.Fatal("IBSSJoin failed: ", err)
	}
	err = iw.IBSSLeave(ctx, iface)
	if err != nil {
		s.Fatal("IBSSLeave failed: ", err)
	}
}
