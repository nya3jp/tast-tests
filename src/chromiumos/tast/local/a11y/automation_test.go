// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package a11y

import (
	"bytes"
	"encoding/json"
	"reflect"
	"regexp"
	"testing"

	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
)

func TestFindParamsRawAttributes(t *testing.T) {
	fp := FindParams{
		Name: "Hello World",
		Attributes: map[string]interface{}{
			"className":   "foo.bar",
			"checked":     checked.True,
			"restriction": restriction.Disabled,
		},
	}

	got, err := fp.rawAttributes()
	if err != nil {
		t.Fatal("rawAttributes failed: ", err)
	}

	var parsed map[string]string
	if err = json.Unmarshal(got, &parsed); err != nil {
		t.Fatal("Unmarshal failed: ", err)
	}

	want := map[string]string{
		"name":        "Hello World",
		"className":   "foo.bar",
		"checked":     "true",
		"restriction": "disabled",
	}
	if !reflect.DeepEqual(parsed, want) {
		t.Fatalf("rawAttributes() = %+v; want %+v", parsed, want)
	}
}

func TestFindParamsRawAttributesRegexp(t *testing.T) {
	r, _ := regexp.Compile("ab+c")
	fp := FindParams{
		Attributes: map[string]interface{}{
			"className": r,
		},
	}

	got, err := fp.rawAttributes()
	if err != nil {
		t.Fatal("rawAttributes failed: ", err)
	}

	want := []byte(`{"className":/ab+c/}`)
	if bytes.Compare(got, want) != 0 {
		t.Fatalf("rawAttributes() = %+v; want %+v", string(got), string(want))
	}
}

func TestFindParamsRawBytes(t *testing.T) {
	fp := FindParams{
		Name: "Hello World",
		Role: role.StaticText,
		State: map[state.State]bool{
			state.Focused: true,
		},
	}

	got, err := fp.rawBytes()
	if err != nil {
		t.Fatal("rawBytes failed: ", err)
	}

	want := []byte(`{"attributes":{"name":"Hello World"},"role":"staticText","state":{"focused":true}}`)
	if bytes.Compare(got, want) != 0 {
		t.Fatalf("rawBytes() = %+v; want %+v", string(got), string(want))
	}
}
