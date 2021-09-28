// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package video provides common code to run ARC binary tests for video encoding.
package video

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kylelemons/godebug/diff"

	"chromiumos/tast/common/perf"
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
		} else {
			diff := diff.Diff(string(data), string(golden))
			t.Fatalf("%v; output:\n%s", err, diff)
		}
	}
}

func TestReportMetrics(t *testing.T) {
	const name = "crowd-1920x1080_h264"
	ctx := context.Background()

	p := perf.NewValues()
	if err := reportFPS(ctx, p, name, "testdata/TestFPS.log"); err != nil {
		t.Error("Failed at reportFPS(): ", err)
	}

	if err := reportEncodeLatency(ctx, p, name, "testdata/TestLatency.log"); err != nil {
		t.Error("Failed at reportEncodeLatency(): ", err)
	}

	if err := reportCPUUsage(ctx, p, name, "testdata/TestCPU.log"); err != nil {
		t.Error("Failed at reportCPUUsage(): ", err)
	}

	saveAndCompare(t, p, "testdata/TestResultsChart.json")
}
