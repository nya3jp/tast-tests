// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"testing"
)

func TestDynamicWriter(t *testing.T) {
	var dw dynamicWriter

	const msg = "message"
	if n, err := dw.Write([]byte(msg)); err != nil {
		t.Errorf("Write(%q) with nil dest failed: %v", msg, err)
	} else if n != len(msg) {
		t.Errorf("Write(%q) with nil dest wrote %d byte(s); want %v", msg, n, len(msg))
	}

	var b1 bytes.Buffer
	dw.setDest(&b1)
	if n, err := dw.Write([]byte(msg)); err != nil {
		t.Errorf("Write(%q) to first buffer failed: %v", msg, err)
	} else if n != len(msg) {
		t.Errorf("Write(%q) to first buffer wrote %d byte(s); want %v", msg, n, len(msg))
	} else if b1.String() != msg {
		t.Fatalf("Write(%q) to first buffer wrote %q; want %q", msg, b1.String(), msg)
	}

	var b2 bytes.Buffer
	dw.setDest(&b2)
	if n, err := dw.Write([]byte(msg)); err != nil {
		t.Errorf("Write(%q) to second buffer failed: %v", msg, err)
	} else if n != len(msg) {
		t.Errorf("Write(%q) to second buffer wrote %d byte(s); want %v", msg, n, len(msg))
	} else if b2.String() != msg {
		t.Fatalf("Write(%q) to second buffer wrote %q; want %q", msg, b2.String(), msg)
	} else if b1.String() != msg {
		t.Errorf("Write(%q) to second buffer made first buffer contain %q; want %q", msg, b1.String(), msg)
	}
}
