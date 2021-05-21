// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hammerd

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TriggeredOnBoot,
		Desc:         "Hammerd smoke test to ensure Hammerd is triggered on boot",
		Contacts:     []string{"fshao@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      1 * time.Minute, // optional
	})
}

func TriggeredOnBoot(ctx context.Context, s *testing.State) {
	// The actual test goes here.
	fmt.Println("Hello world")
	s.Log("Hello world")
}
