// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCalculateCurrentHashes(t *testing.T) {
	hashes, err := CalculateCurrentHashes("/dev/zero", 2)
	if err != nil {
		t.Fatal("CalculateCurrentHashes() failed: ", err)
	}

	exp := []string{
		"3381de4ca9f3a477f25989dfc8b744e7916046b7aa369f61a9a2f7dc0963ec9e",
		"3381de4ca9f3a477f25989dfc8b744e7916046b7aa369f61a9a2f7dc0963ec9e",
	}

	if !cmp.Equal(hashes, exp) {
		t.Errorf("CalculateCurrentHashes() = %+v; want %+v", hashes, exp)
	}
}

func TestWriteRandomData(t *testing.T) {
	file, err := ioutil.TempFile(os.TempDir(), "trim_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	if err := WriteRandomData(file.Name(), 2); err != nil {
		t.Fatal("WriteRandomData() failed: ", err)
	}

	fileInfo, err := os.Stat(file.Name())
	if err != nil {
		t.Fatal("Failed reading temp file stats: ", err)
	}

	actual := fileInfo.Size()
	expected := int64(2 * TrimChunkSize)
	if actual != expected {
		t.Errorf("WriteRandomData() = %d; want %d", actual, expected)
	}
}
