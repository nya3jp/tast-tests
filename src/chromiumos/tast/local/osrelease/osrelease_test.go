// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package osrelease_test

import (
	"bytes"
	"reflect"
	"testing"

	"chromiumos/tast/local/osrelease"
)

func TestParse(t *testing.T) {
	const data = `# Normal line.
SOME_KEY=value
# Key and value with leading/trailing whitespace.
  WS_KEY  =    value
# Value with whitespace in the middle.
WS_VALUE = v a l u e
# Value with quotes don't get removed.
DOUBLE_QUOTES = "double"
SINGLE_QUOTES = 'sin gle'
RANDOM_QUOTES = '"
`
	exp := map[string]string{
		"SOME_KEY":      "value",
		"WS_KEY":        "value",
		"WS_VALUE":      "v a l u e",
		"DOUBLE_QUOTES": `"double"`,
		"SINGLE_QUOTES": `'sin gle'`,
		"RANDOM_QUOTES": `'"`,
	}
	res, err := osrelease.Parse(bytes.NewBufferString(data), nil)
	if err != nil {
		t.Fatal("Parse failed: ", err)
	}
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("Parse returned %+v; want %+v", res, exp)
	}

	overrides := map[string]string{
		"SOME_KEY": "SOME OTHER value",
	}
	exp["SOME_KEY"] = "SOME OTHER value"
	res, err = osrelease.Parse(bytes.NewBufferString(data), overrides)
	if err != nil {
		t.Fatal("Parse failed: ", err)
	}
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("Parse returned %+v; want %+v", res, exp)
	}
}
