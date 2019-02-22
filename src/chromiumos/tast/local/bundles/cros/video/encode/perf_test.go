// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"chromiumos/tast/diff"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
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
		data, _ := ioutil.ReadFile(path)
		golden, _ := ioutil.ReadFile(goldenPath)
		diff, _ := diff.Diff(string(data), string(golden))
		t.Fatalf("%v; output:\n%s", err, diff)
	}
}

func TestReportMetrics(t *testing.T) {
	const name = "crowd-1920x1080_h264"

	p := perf.NewValues()
	if err := reportFPS(p, name, "testdata/TestFPS.log"); err != nil {
		t.Error("Failed at reportFPS(): ", err)
	}

	if err := reportEncodeLatency(p, name, "testdata/TestLatency.log"); err != nil {
		t.Error("Failed at reportEncodeLatency(): ", err)
	}

	if err := reportCPUUsage(p, name, "testdata/TestCPU.log"); err != nil {
		t.Error("Failed at reportCPUUsage(): ", err)
	}

	if err := reportFrameStats(p, name, "testdata/TestFrameStats.log"); err != nil {
		t.Error("Failed at reportFrameStats(): ", err)
	}

	saveAndCompare(t, p, "testdata/TestResultsChart.json")
}
