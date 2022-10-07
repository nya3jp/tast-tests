// Copyright 2020 The ChromiumOS Authors
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
		Func:         LocalFreeze,
		Desc:         "Always freezes",
		Contacts:     []string{"tast-owners@google.com"},
		BugComponent: "b:1034625",
		Timeout:      time.Second,
	})
}

func LocalFreeze(ctx context.Context, s *testing.State) {
	var ch chan struct{}
	<-ch
}
