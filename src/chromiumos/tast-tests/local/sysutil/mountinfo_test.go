// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sysutil

import "testing"

func TestUnescape(t *testing.T) {
	for _, e := range []struct {
		input  string
		expect string
	}{
		{`/media/removable/USB\040Drive`, "/media/removable/USB Drive"},
		{`/media/foo\011bar`, "/media/foo\tbar"},
		{`/media/foo\012bar`, "/media/foo\nbar"},
		{`/media/foo\134bar`, "/media/foo\\bar"},
	} {
		if val, err := unescape(e.input); err != nil {
			t.Errorf("unescape(%q) failed: %v", e.input, err)
		} else if val != e.expect {
			t.Errorf("unescape(%q) failed: got %q; want %q", e.input, val, e.expect)
		}
	}
}
