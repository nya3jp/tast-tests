// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Sanity,
		Desc:         "Tests basic Crostini startup only (where crostini was shipped with the build)",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline"},
		Params:       crostini.MakeTestParams(crostini.TestCritical),
	})
}

func Sanity(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	if err := crostini.SimpleCommandWorks(ctx, cont); err != nil {
		s.Fatal("Failed to run a command in the container: ", err)
	}
}
