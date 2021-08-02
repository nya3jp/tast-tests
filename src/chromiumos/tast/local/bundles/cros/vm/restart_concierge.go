// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RestartConcierge,
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func RestartConcierge(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to login: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Restarting vm_concierge")
	if err := upstart.RestartJob(ctx, "vm_concierge"); err != nil {
		s.Fatal("Restarting vm_concierge failed: ", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	s.Log("Restarting ui")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Restarting ui failed: ", err)
	}
}
