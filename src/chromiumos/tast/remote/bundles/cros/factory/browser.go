// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/factory/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Browser,
		Desc:     "Test if factory UI is running",
		Contacts: []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  time.Minute,
		Pre:      pre.GetToolkitEnsurer(),
		// Skip "nyan_kitty" due to slow reboot speed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kitty")),
	})
}

func Browser(ctx context.Context, s *testing.State) {
	conn := s.DUT().Conn()
	probeDebugPortCmd := conn.CommandContext(ctx, "curl", "localhost:9222/json/list")
	probeResponse, err := probeDebugPortCmd.Output()
	if err != nil {
		s.Fatal("Failed to connect to debugging port: ", err)
	}
	if !bytes.Contains(probeResponse, []byte("\"type\": \"page\"")) {
		s.Fatal("Page is incorrect, probe response: ", string(probeResponse))
	}
}
