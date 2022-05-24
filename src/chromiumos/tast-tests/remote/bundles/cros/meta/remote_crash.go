// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RemoteCrash,
		Desc:     "Always crashes",
		Contacts: []string{"tast-owners@google.com"},
	})
}

func RemoteCrash(ctx context.Context, s *testing.State) {
	// Panic on a goroutine so that it is not recovered by the framework.
	go panic("crashing")
	var ch chan struct{}
	<-ch
}
