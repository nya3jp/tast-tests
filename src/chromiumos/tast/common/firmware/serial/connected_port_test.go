// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial_test

import (
	"context"
	"testing"
	"time"

	"chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/common/testutil"
)

func TestConnectedPort(t *testing.T) {
	if !testutil.InChroot() {
		t.Skip("Needs CrOS SDK (for socat) to run this test")
	}

	ctx := context.Background()

	ctx, ctxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer ctxCancel()
	pty1, pty2, cancel, done, err := serial.CreateHostPTYPair(ctx)
	if err != nil {
		t.Fatal("Error creating pty: ", err)
	}
	t.Logf("Created ptys: %s %s", pty1, pty2)

	defer func() {
		cancel()
		<-done
	}()

	o1 := serial.NewConnectedPortOpener(pty1, 115200, 200*time.Millisecond)
	o2 := serial.NewConnectedPortOpener(pty2, 115200, 200*time.Millisecond)

	t.Log("Opening port should work")
	p1, err := o1.OpenPort(ctx)
	if err != nil {
		t.Fatal("Open port 1", err)
	}
	defer p1.Close(ctx)

	p2, err := o2.OpenPort(ctx)
	if err != nil {
		t.Fatal("Open port 2", err)
	}
	defer p2.Close(ctx)

	if err = serial.DoTestWrite(ctx, t.Log, p1, p2); err != nil {
		t.Fatal("TestWrite failed:", err)
	}

	if err = serial.DoTestFlush(ctx, t.Log, p1, p2); err != nil {
		t.Fatal("TestFlush failed:", err)
	}
}
