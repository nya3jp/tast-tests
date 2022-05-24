// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

import (
	"context"
	"reflect"
	"testing"
)

func TestHostBinaryRunnerParseLDDOutput(t *testing.T) {
	lddOutputString := `	linux-vdso.so.1 (0x00007ffc2996f000)
	libluajit-5.1.so.2 => /usr/local/lib64/libluajit-5.1.so.2 (0x00007f6be5476000)
	libm.so.6 => /lib64/libm.so.6 (0x00007f6be4d43000)
	libpthread.so.0 => /lib64/libpthread.so.0 (0x00007f6be4b25000)
	libc.so.6 => /lib64/libc.so.6 (0x00007f6be476c000)
	libdl.so.2 => /lib64/libdl.so.2 (0x00007f6be4568000)
	/lib64/ld-linux-x86-64.so.2 (0x00007f6be52cc000)
`

	expectedDynLibs := map[string]string{
		"libluajit-5.1.so.2": "/usr/local/lib64/libluajit-5.1.so.2",
		"libm.so.6":          "/lib64/libm.so.6",
		"libpthread.so.0":    "/lib64/libpthread.so.0",
		"libc.so.6":          "/lib64/libc.so.6",
		"libdl.so.2":         "/lib64/libdl.so.2",
	}
	expectedDynLinker := "/lib64/ld-linux-x86-64.so.2"

	dynLibs, dynLinker := parseLddOutput(context.Background(), lddOutputString)
	if !reflect.DeepEqual(dynLibs, expectedDynLibs) {
		t.Errorf("parseLddOutput() returned dynLibs %v; want %v", dynLibs, expectedDynLibs)
	}

	if dynLinker != expectedDynLinker {
		t.Errorf("paresLddOutput() returned dynLinker %q; want %q", dynLinker, expectedDynLinker)
	}
}
