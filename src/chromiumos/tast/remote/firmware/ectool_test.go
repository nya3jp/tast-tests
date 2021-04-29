// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import "testing"

func TestFWImageTypeUnmarshal(t *testing.T) {
	var v FWImageType
	if err := UnmarshalECTool([]byte("RW"), &v); err != nil {
		t.Fatal(err.Error())
	}
	if v != FWImageTypeRW {
		t.Fatal("Failed to parse 'RW'")
	}

	if err := UnmarshalECTool([]byte("RO"), &v); err != nil {
		t.Fatal(err.Error())
	}
	if v != FWImageTypeRO {
		t.Fatal("Failed to parse 'RO'")
	}

	if err := UnmarshalECTool([]byte("unknown"), &v); err != nil {
		t.Fatal(err.Error())
	}
	if v != FWImageTypeUnknown {
		t.Fatal("Failed to parse 'unknown'")
	}
}

func TestECToolVersion(t *testing.T) {
	const ectoolVersionOutput = `
		RO version:    dratini_v2.1.2766-fa4de253e
		RW version:    dratini_v2.0.2766-e28e9252c
		Firmware copy: RW
		Build info:    dratini_v2.0.2766-e28e9252c 2020-03-05 18:16:43 @chromeos-ci-legacy-us-east1-d-x32-71-nxw7
		Tool version:  1.1.9999-eb0d047  @funtop
		`
	var correctVersion = ECToolVersion{
		ROVersion:   "dratini_v2.1.2766-fa4de253e",
		RWVersion:   "dratini_v2.0.2766-e28e9252c",
		Active:      FWImageTypeRW,
		BuildInfo:   "dratini_v2.0.2766-e28e9252c 2020-03-05 18:16:43 @chromeos-ci-legacy-us-east1-d-x32-71-nxw7",
		ToolVersion: "1.1.9999-eb0d047  @funtop",
	}

	var v ECToolVersion
	if err := UnmarshalECTool([]byte(ectoolVersionOutput), &v); err != nil {
		t.Fatal(err.Error())
	}

	if v != correctVersion {
		t.Logf("Correct Version:\n%v", correctVersion)
		t.Logf("Parsed Version:\n%v", v)
		t.Fatal("Parsed version doesn't match correct version.")
	}
}

func TestParseColonDelimited(t *testing.T) {
	const ectoolVersionOutput = `

f1: A B C
	 f2	 : D E F
 f3 : G : H : I


`

	vals := parseColonDelimited(ectoolVersionOutput)
	if keys := len(vals); keys != 3 {
		t.Fatalf("Wrong number of keys. Expected 3, but received %d keys.", keys)
	}
	for k, v := range vals {
		if v == "" {
			t.Fatalf("Key %s had blank value.", k)
		}
	}

	check := func(field, value string) {
		v, ok := vals[field]
		if !ok {
			t.Fatal("Missing field '" + field + "'.")
		}
		if vals[field] != value {
			t.Fatal("Field " + field + " contains invalid entry '" + v + "'.")
		}
		delete(vals, field)
	}

	check("f1", "A B C")
	check("f2", "D E F")
	check("f3", "G : H : I")

	if len(vals) != 0 {
		t.Fatal("Parsed extra values.")
	}
}
