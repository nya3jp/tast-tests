// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encoding

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/diff"
	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

func loadJSON(path string) (interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var v interface{}
	if err := json.NewDecoder(f).Decode(&v); err != nil {
		return nil, errors.Wrapf(err, "failed decoding %s", path)
	}
	return v, nil
}

func jsonEquals(path1, path2 string) error {
	v1, err := loadJSON(path1)
	if err != nil {
		return err
	}
	v2, err := loadJSON(path2)
	if err != nil {
		return err
	}

	if !reflect.DeepEqual(v1, v2) {
		return errors.New("JSON files differ")
	}
	return nil
}

func saveAndCompare(t *testing.T, p *perf.Values, goldenPath string) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	if err := p.Save(td); err != nil {
		t.Fatal("Failed saving JSON: ", err)
	}

	path := filepath.Join(td, "results-chart.json")
	if err := jsonEquals(path, goldenPath); err != nil {
		if data, rerr := ioutil.ReadFile(path); rerr != nil {
			t.Fatal(rerr)
		} else if golden, gerr := ioutil.ReadFile(goldenPath); gerr != nil {
			t.Fatal(gerr)
		} else if diff, derr := diff.Diff(string(data), string(golden)); derr != nil {
			t.Fatal(derr)
		} else {
			t.Fatalf("%v; output:\n%s", err, diff)
		}
	}
}

func TestReportMetrics(t *testing.T) {
	const name = "crowd-1920x1080_h264"

	p := perf.NewValues()
	if err := ReportFPS(p, name, "testdata/TestFPS.log"); err != nil {
		t.Error("Failed at ReportFPS(): ", err)
	}

	if err := ReportEncodeLatency(p, name, "testdata/TestLatency.log"); err != nil {
		t.Error("Failed at ReportEncodeLatency(): ", err)
	}

	if err := ReportCPUUsage(p, name, "testdata/TestCPU.log"); err != nil {
		t.Error("Failed at ReportCPUUsage(): ", err)
	}

	if err := ReportFrameStats(p, name, "testdata/TestFrameStats.log"); err != nil {
		t.Error("Failed at ReportFrameStats(): ", err)
	}

	saveAndCompare(t, p, "testdata/TestResultsChart.json")
}
