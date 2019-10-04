// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/vimcompile"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VimCompile,
		Desc:         "Crostini performance test which compiles vim",
		Contacts:     []string{"sushma.venkatesh.reddy@intel.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:crosbolt","crosbolt_perbuild"},
		Timeout:      12 * time.Minute,
		Pre:          crostini.StartedByDownload(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// VimCompile downloads the VM image from the staging bucket, i.e. it emulates the setup-flow that a user has.
// Compiles vim package 10 times and displays the average amount of time taken to compile vim.
func VimCompile(ctx context.Context, s *testing.State) {
	vimcompile.RunTest(ctx, s, s.PreValue().(crostini.PreData).Container)
}