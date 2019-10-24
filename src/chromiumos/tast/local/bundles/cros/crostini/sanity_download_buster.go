// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/sanity"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SanityDownloadBuster,
		Desc:         "Tests basic Crostini startup only (where a buster release of crostini was downloaded first)",
		Contacts:     []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
		Pre:          crostini.StartedByDownloadBuster(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func SanityDownloadBuster(ctx context.Context, s *testing.State) {
	sanity.RunTest(ctx, s, s.PreValue().(crostini.PreData).Container)
}
