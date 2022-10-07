// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalFreezeForever,
		Desc:         "Always freezes forever",
		Contacts:     []string{"tast-owners@google.com"},
		BugComponent: "b:1034625",
		Timeout:      100 * time.Hour,
	})
}

func LocalFreezeForever(ctx context.Context, s *testing.State) {
	// Log for meta test to detect start of the test.
	s.Log("LocalFreezeForever started")
	var ch chan struct{}
	<-ch
}
