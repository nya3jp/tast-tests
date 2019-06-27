// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"

	"chromiumos/tast/local/bundles/cros/network/iw"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	phy   = "phy0"
	iface = "mon0"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IWInterfaceConnection,
		Desc:     "Verifies `iw` interface connection behavior",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func IWInterfaceConnection(ctx context.Context, s *testing.State) {
	if err := iw.AddInterface(ctx, phy, iface, "monitor"); err != nil {
		s.Fatal("AddInterface failed: ", err)
	}
	defer func() {
		if err := iw.RemoveInterface(ctx, iface); err != nil {
			s.Error("RemoveInterface failed: ", err)
		}
	}()
	if err := testexec.CommandContext(ctx, "ifconfig", iface, "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal(fmt.Sprintf("Could not bring up interface %s: ", iface), err)
	}
}
