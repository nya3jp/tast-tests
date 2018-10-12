// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

func loadJson(path string) (interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed opening %s", path)
	}
	defer f.Close()

	var v interface{}
	if err := json.NewDecoder(f).Decode(&v); err != nil {
		return nil, errors.Wrapf(err, "failed decoding %s", path)
	}
	return v, nil
}

func jsonEquals(path1, path2 string) error {
	v1, err := loadJson(path1)
	if err != nil {
		return err
	}
	v2, err := loadJson(path2)
	if err != nil {
		return err
	}

	if !reflect.DeepEqual(v1, v2) {
		return errors.New("JSON files differ")
	}
	return nil
}

func saveAndCompare(t *testing.T, p *Values, goldenPath string) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	if err := p.Save(td); err != nil {
		t.Fatal("Failed saving JSON: ", err)
	}

	if err := jsonEquals(filepath.Join(td, "results-chart.json"), goldenPath); err != nil {
		t.Fatal(err)
	}
}

func TestSetSingle(t *testing.T) {
	metric := Metric{Name: "metric", Unit: "unit", Direction: SmallerIsBetter}
	var p Values

	p.Set(metric, 1)
	p.Set(metric, 2)
	p.Set(metric, 3)

	saveAndCompare(t, &p, "testdata/TestSetSingle.json")
}

func TestSetSinglePanic(t *testing.T) {
	metric := Metric{Name: "metric", Unit: "unit", Direction: SmallerIsBetter}
	var p Values

	defer func() {
		if r := recover(); r == nil {
			t.Error("Did not panic")
		}
	}()

	// Set with multiple values panics for single-valued metrics.
	p.Set(metric, 1, 2, 3)
}

func TestSetMultiple(t *testing.T) {
	metric := Metric{Name: "metric", Unit: "unit", Direction: SmallerIsBetter, Multiple: true}
	var p Values

	p.Set(metric, 1, 2, 3)
	p.Set(metric, 4, 5, 6)

	saveAndCompare(t, &p, "testdata/TestSetMultiple.json")
}

func TestAppendSinglePanic(t *testing.T) {
	metric := Metric{Name: "metric", Unit: "unit", Direction: SmallerIsBetter}
	var p Values

	defer func() {
		if r := recover(); r == nil {
			t.Error("Did not panic")
		}
	}()

	// Append panics for single-valued metrics.
	p.Append(metric, 1)
}

func TestAppendMultiple(t *testing.T) {
	metric := Metric{Name: "metric", Unit: "unit", Direction: SmallerIsBetter, Multiple: true}
	var p Values

	p.Append(metric, 1)
	p.Append(metric, 2, 3)

	saveAndCompare(t, &p, "testdata/TestAppendMultiple.json")
}

func TestSave(t *testing.T) {
	var (
		metric1  = Metric{Name: "metric1", Unit: "unit1", Direction: SmallerIsBetter}
		metric2  = Metric{Name: "metric2", Unit: "unit2", Direction: SmallerIsBetter, Multiple: true}
		metric3a = Metric{Name: "metric3", Variant: "a", Unit: "unit3a", Direction: SmallerIsBetter}
		metric3b = Metric{Name: "metric3", Variant: "b", Unit: "unit3b", Direction: BiggerIsBetter}
	)

	var p Values
	p.Set(metric1, 100)
	p.Set(metric2, 200, 201, 202)
	p.Set(metric3a, 300)
	p.Set(metric3b, 310)

	saveAndCompare(t, &p, "testdata/TestSave.json")
}
