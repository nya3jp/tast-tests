// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"bytes"
	"testing"
	"time"
)

type CloseBuffer bytes.Buffer

func (b *CloseBuffer) Write(p []byte) (int, error) { return (*bytes.Buffer)(b).Write(p) }

func (b *CloseBuffer) Close() error { return nil }

func TestEventWriter(t *testing.T) {
	b := CloseBuffer{}
	ew := newWriter(&b, nil)
	now := time.Unix(1, 0)
	ew.Event(now, EV_KEY, KEY_A, 1)
	ew.Event(now, EV_KEY, KEY_A, 0)
	ew.Sync(now)
	if err := ew.Close(); err != nil {
		t.Error("EventWriter reported error: ", err)
	}

	// FIXME: Check written events.
}
