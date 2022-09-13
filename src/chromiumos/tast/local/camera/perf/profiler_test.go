// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"fmt"
	"testing"

	"chromiumos/tast/common/perf"
)

func TestConvertFromProtobuf(t *testing.T) {
	pv1 := perf.NewValues()
	const pv1Name = "pv1_metric"
	const pv1Unit = "J"
	const pv1Value = 0.1
	pv1.Set(perf.Metric{
		Name:      pv1Name,
		Unit:      pv1Unit,
		Direction: perf.SmallerIsBetter,
	}, pv1Value)

	pv2 := perf.NewValues()
	const pv2Name = "pv2_metric"
	const pv2Unit = "count"
	const pv2Value = 10
	pv2.Set(perf.Metric{
		Name:      pv2Name,
		Unit:      pv2Unit,
		Direction: perf.BiggerIsBetter,
	}, pv2Value)

	result := perf.NewValues()
	const prefix = "foo"
	convertFromProtobuf(pv1.Proto().Values, result, prefix)
	convertFromProtobuf(pv2.Proto().Values, result, prefix)

	if len(result.Proto().Values) != 2 {
		t.Errorf("Unexpected length of merged perf values: got %v, want %v", len(result.Proto().Values), 2)
	}
	for _, v := range result.Proto().Values {
		switch v.Name {
		case fmt.Sprintf("%s-%s", prefix, pv1Name):
			if v.Unit != pv1Unit {
				t.Errorf("Unexpected %q unit: got %v, want %v", pv1Name, v.Unit, pv1Unit)
			} else if v.Value[0] != pv1Value {
				t.Errorf("Unexpected %q value: got %v, want %v", pv1Name, v.Value, pv1Unit)
			}
		case fmt.Sprintf("%s-%s", prefix, pv2Name):
			if v.Unit != pv2Unit {
				t.Errorf("Unexpected %q unit: got %v, want %v", pv2Name, v.Unit, pv2Unit)
			} else if v.Value[0] != pv2Value {
				t.Errorf("Unexpected %q value: got %v, want %v", pv2Name, v.Value, pv2Unit)
			}
		default:
			t.Errorf("unexpected metric %v", v)
		}
	}
}
