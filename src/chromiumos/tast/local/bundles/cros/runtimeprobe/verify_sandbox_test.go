// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package runtimeprobe

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestProbeConfigMarshal tests the json encoding of probe config.
func TestProbeConfigMarshal(t *testing.T) {
	var expectedResult = []byte(
		`{"Func1_category":{"Func1_name":{"eval":{"Func1":{}}}},` +
			`"Func2_category":{"Func2_name":{"eval":{"Func2":{}}}},` +
			`"Func3_category":{"Func3_name":{"eval":{"Func3":{}}}}}`)

	pc := probeConfig{[]probeStatement{
		probeStatement{"Func1"},
		probeStatement{"Func2"},
		probeStatement{"Func3"},
	}}
	b, err := json.Marshal(pc)
	if err != nil {
		t.Fatal("Failed to marshal probe config: ", err)
	}
	if diff := cmp.Diff(b, expectedResult); diff != "" {
		t.Fatalf("probeConfig Marshal mismatch (-want +got):\n%s", diff)
	}
}
