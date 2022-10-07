// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalKernelPanic,
		Desc:         "Triggers an intentional kernel panic with sysrq",
		Contacts:     []string{"tast-owners@google.com"},
		BugComponent: "b:1034625",
	})
}

func LocalKernelPanic(ctx context.Context, s *testing.State) {
	// Save changes to the disk before triggering a kernel panic to avoid losing files.
	if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("sync failed: ", err)
	}

	// Trigger a kernel panic. Don't try this at home.
	ioutil.WriteFile("/proc/sysrq-trigger", []byte("c"), 0666)

	// Wait until the test timeout is reached.
	<-ctx.Done()
}
