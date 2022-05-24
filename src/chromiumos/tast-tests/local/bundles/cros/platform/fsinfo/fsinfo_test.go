// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fsinfo

import (
	"reflect"
	"testing"
)

func TestParseDfOutput(t *testing.T) {
	const out = `
Filesystem     Type  1B-blocks       Used Available Use% Mounted on
/dev/root      ext2 2064203776 1718423552 345780224  84% /
`
	info, err := parseDfOutput([]byte(out))
	if err != nil {
		t.Fatal("parseDfOutput() failed: ", err)
	}
	exp := Info{
		Dev:   "/dev/root",
		Type:  "ext2",
		Used:  1718423552,
		Avail: 345780224,
	}
	if !reflect.DeepEqual(*info, exp) {
		t.Errorf("parseDfOutput() = %+v; want %+v", *info, exp)
	}
}
