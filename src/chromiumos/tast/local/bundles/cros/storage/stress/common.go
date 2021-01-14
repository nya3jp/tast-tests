// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"chromiumos/tast/errors"
)

const keyValFileName = "keyval"

// WriteKeyVals writes given key value data to an external file in output directory.
func WriteKeyVals(outDir string, keyVals map[string]float64) error {
	if keyVals == nil {
		return errors.New("invalid data to write to keyval file")
	}

	filename := filepath.Join(outDir, keyValFileName)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open file: %s", filename)
	}
	defer f.Close()

	// Sorting all results by name before writing to file.
	keys := make([]string, 0, len(keyVals))
	for k := range keyVals {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if _, err := fmt.Fprintf(f, "%s=%v\n", key, keyVals[key]); err != nil {
			return errors.Wrap(err, "failed to write keyval file")
		}
	}
	return nil
}

// WriteTestStatusFile writes test status JSON file to test's output folder.
// Status file contains start/end times and final test status (passed/failed).
func WriteTestStatusFile(ctx context.Context, outDir string, passed bool, startTimestamp time.Time) error {
	status := struct {
		Started  string `json:"started"`
		Finished string `json:"finished"`
		Passed   bool   `json:"passed"`
	}{
		Started:  startTimestamp.Format(time.RFC3339),
		Finished: time.Now().Format(time.RFC3339),
		Passed:   passed,
	}

	content, err := json.MarshalIndent(status, "", " ")
	if err != nil {
		return errors.Wrap(err, "failed marshalling test status to JSON")
	}
	filename := filepath.Join(outDir, "status.json")
	if err := ioutil.WriteFile(filename, content, 0644); err != nil {
		return errors.Wrap(err, "failed saving test status to file")
	}
	return nil
}
