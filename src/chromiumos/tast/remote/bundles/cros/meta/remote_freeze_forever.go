// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func:     RemoteFreezeForever,
		Desc:     "Always freezes forever",
		Contacts: []string{"tast-owners@google.com"},
		Timeout:  10 * time.Minute,
	})
}

func RemoteFreezeForever(ctx context.Context, s *testing.State) {
	// Log for meta test to detect start of the test.
	s.Log("RemoteFreezeForever started")
	var ch chan struct{}
	<-ch
}
