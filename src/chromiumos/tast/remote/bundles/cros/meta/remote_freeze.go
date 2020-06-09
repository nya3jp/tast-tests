// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:     RemoteFreeze,
		Desc:     "Always freezes",
		Contacts: []string{"tast-owners@google.com"},
		Timeout:  time.Second,
	})
}

func RemoteFreeze(ctx context.Context, s *testing.State) {
	var ch chan struct{}
	<-ch
}
