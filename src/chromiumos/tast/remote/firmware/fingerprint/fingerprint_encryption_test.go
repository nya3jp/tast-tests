// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import "testing"

func TestEncryptionStatusEctoolUnmarshalerTPMSeed(t *testing.T) {
	// It should not matter that there are tabs to the left of the examples out.
	var out = `
	FPMCU encryption status: 0x00000001 FPTPM_seed_set
	Valid flags:             0x00000001 FPTPM_seed_set
	`
	var expect = EncryptionStatus{
		Current: EncryptionStatusTPMSeedSet,
		Valid:   EncryptionStatusTPMSeedSet,
	}

	actual, err := unmarshalEctoolEncryptionStatus(out)
	if err != nil {
		t.Fatal("Failed to unmarshal encryption status: ", err)
	}

	if actual != expect {
		t.Fatalf("Unmarshaled encryption status block %+v doesn't match expected block %+v.", actual, expect)
	}
}

func TestEncryptionStatusEctoolUnmarshalerNoTPMSeed(t *testing.T) {
	// It should not matter that there are tabs to the left of the examples out.
	var out = `
	FPMCU encryption status: 0x00000000
	Valid flags:             0x00000001 FPTPM_seed_set
	`
	var expect = EncryptionStatus{
		Current: 0,
		Valid:   EncryptionStatusTPMSeedSet,
	}

	actual, err := unmarshalEctoolEncryptionStatus(out)
	if err != nil {
		t.Fatal("Failed to unmarshal encryption status: ", err)
	}

	if actual != expect {
		t.Fatalf("Unmarshaled encryption status block %+v doesn't match expected block %+v.", actual, expect)
	}
}

func TestEncryptionStatusFlagsIsSet(t *testing.T) {
	var flags EncryptionStatusFlags

	if flags.IsSet(EncryptionStatusTPMSeedSet) {
		t.Fatal("Flag EncryptionStatusTPMSeedSet was reported as set.")
	}

	flags = EncryptionStatusTPMSeedSet
	if !flags.IsSet(EncryptionStatusTPMSeedSet) {
		t.Fatal("Flag EncryptionStatusTPMSeedSet was reported as not set.")
	}
}
