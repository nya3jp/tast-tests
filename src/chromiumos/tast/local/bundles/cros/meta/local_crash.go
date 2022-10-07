// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalCrash,
		Desc:         "Always crashes",
		Contacts:     []string{"tast-owners@google.com"},
		BugComponent: "b:1034625",
	})
}

func LocalCrash(ctx context.Context, s *testing.State) {
	// Panic on a goroutine so that it is not recovered by the framework.
	go panic("crashing")
	var ch chan struct{}
	<-ch
}
