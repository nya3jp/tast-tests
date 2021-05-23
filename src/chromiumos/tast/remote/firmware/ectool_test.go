// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import "testing"

func TestParseColonDelimited(t *testing.T) {
	const ectoolVersionOutput = `
RO version:    dratini_v2.1.2766-fa4de253e
RW version:    dratini_v2.0.2766-e28e9252c
Firmware copy: RW
Build info:    dratini_v2.0.2766-e28e9252c 2020-03-05 18:16:43 @chromeos-ci-legacy-us-east1-d-x32-71-nxw7
Tool version:  1.1.9999-eb0d047  @funtop
`

	vals := parseColonDelimited(ectoolVersionOutput)
	if keys := len(vals); keys != 5 {
		t.Fatalf("Wrong number of keys. Expected 5, but received %d keys.", keys)
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

	check("RO version", "dratini_v2.1.2766-fa4de253e")
	check("RW version", "dratini_v2.0.2766-e28e9252c")
	check("Firmware copy", "RW")
	check("Build info", "dratini_v2.0.2766-e28e9252c 2020-03-05 18:16:43 @chromeos-ci-legacy-us-east1-d-x32-71-nxw7")
	check("Tool version", "1.1.9999-eb0d047  @funtop")

	if len(vals) != 0 {
		t.Fatal("Parsed too many values.")
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
		Active:      FWImageTypeRW,
		ROVersion:   "dratini_v2.1.2766-fa4de253e",
		RWVersion:   "dratini_v2.0.2766-e28e9252c",
		BuildInfo:   "dratini_v2.0.2766-e28e9252c 2020-03-05 18:16:43 @chromeos-ci-legacy-us-east1-d-x32-71-nxw7",
		ToolVersion: "1.1.9999-eb0d047  @funtop",
	}

	var ver ECToolVersion
	if err := Unmarshal([]byte(ectoolVersionOutput), &ver); err != nil {
		t.Fatal("Received error on Unmarshal: ", err)
	}

	if ver != correctVersion {
		t.Logf("Correct Version: %#v", correctVersion)
		t.Logf("Parsed Version: %#v", ver)
		t.Fatal("Parsed EC Version does not match correct version.")
	}
}
