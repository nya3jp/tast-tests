// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"bytes"
	"testing"
)

func TestModifyVersionString(t *testing.T) {
	for _, tc := range []struct {
		suffix          string
		version         []byte
		expectedVersion []byte
		expectErr       bool
	}{
		{".dev", []byte("nocturne_fp_v2.0.7304-441100b93\x00"), []byte("nocturne_fp_v2.0.7304-44110.dev\x00"), false},
		// version too short
		{"", []byte("012345678901234567890123456789"), []byte{}, true},
		// version too long
		{"", []byte("012345678901234567890123456789012"), []byte{}, true},
		// suffix too long
		{"01234567890123456789012345678901", []byte("nocturne_fp_v2.0.7304-441100b93\x00"), []byte{}, true},
		// suffix exactly fits
		{"0123456789012345678901234567890", []byte("nocturne_fp_v2.0.7304-441100b93\x00"), []byte("0123456789012345678901234567890\x00"), false},
		// null terminators in string
		{".dev", []byte("nami_fp_v2.2.144-7a08e07eb\x00\x00\x00\x00\x00\x00"), []byte("nami_fp_v2.2.144-7a08e07eb.dev\x00\x00"), false},
	} {
		actualVersion, err := createVersionStringWithSuffix(tc.suffix, tc.version)
		if err != nil && !tc.expectErr {
			t.Errorf("createVersionStringWithSuffix(%q, %q) returned unexpected error: %v", tc.suffix, tc.version, err)
			return
		}
		if err == nil && tc.expectErr {
			t.Errorf("createVersionStringWithSuffix(%q, %q) unexpectedly succeeded", tc.suffix, tc.version)
			return
		}
		if string(actualVersion) != string(tc.expectedVersion) {
			t.Errorf("createVersionStringWithSuffix(%q, %q) returned %q; want %q", tc.suffix, tc.version, string(actualVersion), string(tc.expectedVersion))
		}
	}
}

func TestCreateRollbackBytes(t *testing.T) {
	for _, tc := range []struct {
		rollbackValue uint32
		expectedBytes []byte
	}{
		{0, []byte{0x00, 0x00, 0x00, 0x00}},
		{1, []byte{0x01, 0x00, 0x00, 0x00}},
		{9, []byte{0x09, 0x00, 0x00, 0x00}},
	} {
		actualBytes := createRollbackBytes(tc.rollbackValue)
		if !bytes.Equal(actualBytes, tc.expectedBytes) {
			t.Errorf("createRollbackBytes(%q) returned %q; want %q", tc.rollbackValue, actualBytes, tc.expectedBytes)
		}
	}
}
