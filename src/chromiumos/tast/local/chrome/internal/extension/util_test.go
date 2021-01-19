// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package extension

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"chromiumos/tast/testutil"
)

func TestComputeExtensionIDFromPublicKey(t *testing.T) {
	dir := testutil.TempDir(t)
	defer os.RemoveAll(dir)

	// Taken from Chrome's components/crx_file/id_util_unittest.cc.
	manifest := `{ "key": "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC4fysg3HybDNxRYZkNNg/UZogIVYTVOr8rpGSFewwEEz+N9Lw4DUn+a8RasEBTOtdmCQ+eNnQw2ooxTx8UUNfHIJQX3k65V15+CuWyZXqJTrZH/xy9tzgTr0eFhDIz8xdJv+mW0NYUbxONxfwscrqs6n4YU1amg6LOk5PnHw/mDwIDAQAB" }`
	if err := ioutil.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	id, err := ComputeExtensionID(dir)
	if err != nil {
		t.Fatalf("ComputeExtensionID(%q) failed with %v", dir, err)
	}
	exp := "melddjfinppjdikinhbgehiennejpfhp"
	if id != exp {
		t.Errorf("ComputeExtensionID(%q) = %q; want %q", dir, id, exp)
	}
}

func TestComputeExtensionIDFromDirName(t *testing.T) {
	// Taken from Chrome's components/crx_file/id_util_unittest.cc.
	for _, tc := range []struct{ dir, exp string }{
		{"test", "jpignaibiiemhngfjkcpokkamffknabf"},
		{"_", "ncocknphbhhlhkikpnnlmbcnbgdempcd"},
	} {
		if id, err := ComputeExtensionID(tc.dir); err != nil {
			t.Errorf("ComputeExtensionID(%q) failed with %v", tc.dir, err)
		} else if id != tc.exp {
			t.Errorf("ComputeExtensionID(%q) = %q; want %q", tc.dir, id, tc.exp)
		}
	}
}
