// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"testing"
)

func TestReplaceCrashFilterString(t *testing.T) {
	for _, i := range []struct {
		old      string
		param    string
		success  bool
		expected string
	}{
		// replace
		{"|/sbin/crash_reporter --filter_in=old_exec --user=%P:%s:%u:%g:%e", "new_exec", true,
			"|/sbin/crash_reporter --filter_in=new_exec --user=%P:%s:%u:%g:%e"},
		// remove
		{"|/sbin/crash_reporter --filter_in=old_exec --user=%P:%s:%u:%g:%e", "", true,
			"|/sbin/crash_reporter --user=%P:%s:%u:%g:%e"},
		// add
		{"|/sbin/crash_reporter --user=foo", "new_exec", true,
			"|/sbin/crash_reporter --user=foo --filter_in=new_exec"},
		// no flags before addition
		{"|/sbin/crash_reporter", "new_exec", true,
			"|/sbin/crash_reporter --filter_in=new_exec"},
		// no flags after removal
		{"|/sbin/crash_reporter --filter_in=old_exec", "", true,
			"|/sbin/crash_reporter"},
		// core pattern is not a piping
		{"core", "new_exec", false, ""},
	} {
		newFilter, err := ReplaceCrashFilterString(i.old, i.param)
		if err != nil && i.success {
			t.Errorf("ReplaceCrashFilterString returned unexpected error: %v", err)
		} else if err == nil && !i.success {
			t.Errorf("ReplaceCrashFilterString should return error: %v", i)
		} else if newFilter != i.expected {
			t.Errorf("ReplaceCrashFilterString result: want %s, got %s", i.expected, newFilter)
		}
	}
}
