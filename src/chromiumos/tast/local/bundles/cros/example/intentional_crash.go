// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IntentionalCrash,
		Desc: "Causes some intentional crashes",
	})
}

func IntentionalCrash(ctx context.Context, s *testing.State) {
	testexec.CommandContext(ctx, "killall", "-SEGV", "powerd").Run()
	testexec.CommandContext(ctx, "killall", "-SEGV", "chrome").Run()
	testexec.CommandContext(ctx, "killall", "-SEGV", "debugd").Run()
	time.Sleep(15 * time.Second)
}
