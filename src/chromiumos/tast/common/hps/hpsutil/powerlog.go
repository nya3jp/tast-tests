// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hpsutil contains functionality used by the HPS tast tests.
package hpsutil

import (
	"bufio"
	"context"
	"encoding/csv"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// PowerlogResult returns all the power samples for all the columns returned by running powerlog.
type PowerlogResult struct {
	Value map[string][]float64
	Err   error
}

func parsePowerlogCsv(ctx context.Context, csvFilename string) PowerlogResult {
	f, err := os.Open(csvFilename)
	if err != nil {
		return PowerlogResult{Err: errors.Wrapf(err, "unable to read input file %s", csvFilename)}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "IndexError: list index out of range") {
			return PowerlogResult{Err: errors.New("Sweetberry not found (replug the USB)")}
		}
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return PowerlogResult{Err: errors.Wrapf(err, "unable to seek %s to 0", csvFilename)}
	}
	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		return PowerlogResult{Err: errors.Wrapf(err, "unable to parse %s as CSV", csvFilename)}
	}
	if len(records) == 0 {
		return PowerlogResult{Err: errors.New("CSV is empty, replug the Sweetberry")}
	}

	// column number to column name.
	names := make(map[int]string)
	// column name to slice of data.
	data := make(map[string][]float64)
	for i, name := range records[0] {
		names[i] = strings.TrimSpace(name)
	}

	// TODO: calculate SPI by subtracting everything from TOTAL_POWER
	for _, row := range records[1:] {
		for i, strValue := range row {
			column := names[i]
			v, err := strconv.ParseFloat(strings.TrimSpace(strValue), 64)
			if err != nil {
				return PowerlogResult{Err: errors.Wrap(err, "unable to parse float")}
			}
			data[column] = append(data[column], v)
		}
	}

	return PowerlogResult{Value: data}
}

// Powerlog runs sweetberry power measurement for specified boardFilename and scenarioFilename, saves the resulting csvFilename, and parses it to the PowerlogResult.
func Powerlog(ctx context.Context, csvFilename, boardFilename, scenarioFilename string, quit chan struct{}, result chan PowerlogResult) {
	{
		cmd := testexec.CommandContext(ctx,
			"powerlog",
			"-b", boardFilename,
			"-c", scenarioFilename)
		testing.ContextLog(ctx, cmd)

		csvFile, err := os.OpenFile(csvFilename,
			os.O_WRONLY|os.O_CREATE|os.O_APPEND,
			0644)
		if err != nil {
			result <- PowerlogResult{Err: errors.Wrapf(err, "cannot open file %s", csvFilename)}
			return
		}
		defer csvFile.Close()
		cmd.Stderr = csvFile
		cmd.Stdout = csvFile
		if err := cmd.Start(); err != nil {
			result <- PowerlogResult{Err: errors.Wrapf(err, "see %s for more details", csvFilename)}
			return
		}

		done := false
		for !done {
			select {
			case <-quit:
				testing.ContextLog(ctx, "quitting powerlog")
				done = true
			}
		}

		testing.ContextLog(ctx, "quit powerlog")
		if err := cmd.Kill(); err != nil {
			result <- PowerlogResult{Err: errors.Wrap(err, "failed to kill powerlog")}
			return
		}
	}

	result <- parsePowerlogCsv(ctx, csvFilename)
}
