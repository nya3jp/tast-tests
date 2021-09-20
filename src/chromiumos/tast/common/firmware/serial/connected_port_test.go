// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"
	"testing"
	"time"
)

func TestConnectedPort(t *testing.T) {
	ctx := context.Background()

	ctxT, ctxCancel := context.WithTimeout(ctx, 2*time.Second)
	defer ctxCancel()
	pty1, pty2, cancel, done, err := CreatePtyPair(ctxT, nil)
	if err != nil {
		t.Fatal("Error creating pty: ", err)
	}
	t.Logf("Created ptys: %s %s", pty1, pty2)

	defer func() {
		cancel()
		<-done
	}()

	o1 := NewConnectedPortOpener(pty1, 115200, 10*time.Millisecond)
	o2 := NewConnectedPortOpener(pty2, 115200, 10*time.Millisecond)

	err = DoTestPort(ctx, t.Log, o1, o2)

	if err != nil {
		t.Fatal("Test failed:", err)
	}
}
