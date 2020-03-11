// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/local/bundles/cros/network/iwlwifirescan"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IwlwifiPCIRescan,
		Desc:         "Verifies that the WiFi interface will recover if removed when the device has iwlwifi_rescan",
		Contacts:     []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		SoftwareDeps: []string{"iwlwifi_rescan"},
		// For now, we prefer the remote version. Disable and keep the test to reproduce issue locally.
	})
}

func IwlwifiPCIRescan(ctx context.Context, s *testing.State) {
	if err := iwlwifirescan.RemoveIfaceAndWaitForRecovery(ctx); err != nil {
		s.Fatal("Test failed with reason: ", err)
	}
}
