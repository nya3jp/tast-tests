// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     SleepRemote,
		Desc:     "Demonstrates connecting to and disconnecting from DUT",
		Contacts: []string{"tast-owners@google.com"},
		Attr:     []string{"disabled"},
		Timeout:  2 * time.Minute,
	})
}

func SleepRemote(ctx context.Context, s *testing.State) {
	time.Sleep(90 * time.Second)
}
