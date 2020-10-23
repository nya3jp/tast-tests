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
	"time"

	"chromiumos/tast/errors"
)

const keyValFileName = "keyval"

// WriteKeyVals writes given key value data to an external file in output directory.
func WriteKeyVals(outDir string, keyVals map[string]int) error {
	filename := filepath.Join(outDir, keyValFileName)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open file: %s", filename)
	}
	defer f.Close()

	for key, val := range keyVals {
		if _, err := f.WriteString(fmt.Sprintf("%s=%d\n", key, val)); err != nil {
			return errors.Wrap(err, "failed to write keyval file")
		}
	}
	return nil
}

// WriteTestStatusFile writes test status JSON file to test's output folder.
// Status file contains start/end times and final test status (passed/failed).
func WriteTestStatusFile(ctx context.Context, outDir string, passed bool, startTimestamp time.Time) error {
	statusFileStruct := struct {
		Started  string `json:"started"`
		Finished string `json:"finished"`
		Passed   bool   `json:"passed"`
	}{
		Started:  startTimestamp.Format(time.RFC3339),
		Finished: time.Now().Format(time.RFC3339),
		Passed:   passed,
	}

	file, err := json.MarshalIndent(statusFileStruct, "", " ")
	if err != nil {
		return errors.Wrap(err, "failed marshalling test status to JSON")
	}
	filename := filepath.Join(outDir, "status.json")
	if err := ioutil.WriteFile(filename, file, 0644); err != nil {
		return errors.Wrap(err, "failed saving test status to file")
	}
	return nil
}
