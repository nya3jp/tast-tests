// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/copypaste"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CopyPasteWaylandToWaylandDownload,
		Desc:         "Tests wayland copy paste functionality",
		Contacts:     []string{"sidereal@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Data:         []string{},
		Pre:          crostini.StartedByDownload(),
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CopyPasteWaylandToWaylandDownload(ctx context.Context, s *testing.State) {
	copypaste.RunTest(ctx, s,
		copypaste.Wayland, "text/plain;charset=utf-8", "Some data to copy",
		copypaste.Wayland, "text/plain;charset=utf-8", "Some data to copy")
}
