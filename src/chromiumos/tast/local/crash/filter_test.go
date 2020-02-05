// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"testing"
)

func TestReplaceArgs(t *testing.T) {
	for _, i := range []struct {
		input    string
		prefix   string
		newarg   string
		expected string
	}{
		{"|/sbin/crash_reporter --user=x", "-v=", "-v=1", "|/sbin/crash_reporter --user=x -v=1"},
		{"|/sbin/crash_reporter --user=x -v=2 -foo", "-v=", "-v=1", "|/sbin/crash_reporter --user=x -v=1 -foo"},
		{"|/sbin/crash_reporter --user=x -v=1 -foo", "-v=", "", "|/sbin/crash_reporter --user=x -foo"},
		{"|/sbin/crash_reporter --filter_in=foo", "--filter_in=", "", "|/sbin/crash_reporter"},
	} {
		r := replaceArgs(i.input, i.prefix, i.newarg)
		if r != i.expected {
			t.Errorf("Replace(%v) = %v; want %v", i.input, r, i.expected)
		}
	}
}
