// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cdputil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"chromiumos/tast/testutil"
)

func TestReadDebuggingPort(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	for _, tc := range []struct {
		name, data string
		port       int // expected port or -1 if error is expected
	}{
		{"full", "56245\n/devtools/browser/01db187e-2e2a-42c5-833e-bd4dbea9e313", 56245},
		{"oneline", "123", 123},
		{"garbage", "foo", -1},
		{"empty", "", -1},
	} {
		p := filepath.Join(td, tc.name)
		if err := ioutil.WriteFile(p, []byte(tc.data), 0644); err != nil {
			t.Fatal(err)
		}
		port, err := readDebuggingPort(p)
		if tc.port == -1 {
			if err == nil {
				t.Errorf("readDebuggingPort(%q) (data %q) didn't return expected error", tc.name, tc.data)
			}
		} else {
			if err != nil {
				t.Errorf("readDebuggingPort(%q) (data %q) returned error: %v", tc.name, tc.data, err)
			} else if port != tc.port {
				t.Errorf("readDebuggingPort(%q) (data %q) = %d; want %d", tc.name, tc.data, port, tc.port)
			}
		}
	}

	if _, err := readDebuggingPort(filepath.Join(td, "missing")); err == nil {
		t.Error("readDebuggingPort didn't return expected error for missing file")
	}
}
