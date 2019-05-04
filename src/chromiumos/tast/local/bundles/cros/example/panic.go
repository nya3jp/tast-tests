// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Panic,
		Desc:     "Causes a panic in a goroutine to crash the bundle process",
		Contacts: []string{"tast-owners@chromium.org"},
	})
}

func Panic(ctx context.Context, s *testing.State) {
	done := make(chan struct{})
	go func() {
		panic("Intentional panic")
		close(done)
	}()

	<-done
	s.Fatal("Unexpectedly succeeded")
}
