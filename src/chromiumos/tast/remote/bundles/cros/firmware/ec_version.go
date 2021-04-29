// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECVersion,
		Desc:         "Verify that the EC version can be retrieved from ectool",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECVersion(ctx context.Context, s *testing.State) {
	ec := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
	version, err := ec.Version(ctx)
	if err != nil {
		s.Fatal("Failed to determine EC version: ", err)
	}
	s.Log("EC version: ", version)
}
