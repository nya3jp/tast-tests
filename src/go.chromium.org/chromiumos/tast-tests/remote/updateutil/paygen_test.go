// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateutil

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	wantLen := 5

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if paygen, err := parsePaygen(ctx, "testdata/TestPaygen.json"); err != nil {
		t.Error("Failed to load the testfile")
	} else if len(paygen.Deltas) != wantLen {
		t.Errorf("Unexpected number of entries; got %d, want %d", len(paygen.Deltas), wantLen)
	}
}

func TestFilter(t *testing.T) {
	wantLenFilter1 := 3
	wantLenFilter2 := 1

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	paygen, err := parsePaygen(ctx, "testdata/TestPaygen.json")
	if err != nil {
		t.Error("Failed to load the testfile")
	}

	filtered := paygen.FilterBoardChannelDeltaType("sarien", "canary", "OMAHA")
	if len(filtered.Deltas) != wantLenFilter1 {
		t.Errorf("Unexpected number of entries after 1st filter; got %d, want %d", len(filtered.Deltas), wantLenFilter1)
	}

	filtered = paygen.FilterMilestone(97)
	if len(filtered.Deltas) != wantLenFilter2 {
		t.Errorf("Unexpected number of entries after 2nd filter; got %d, want %d", len(filtered.Deltas), wantLenFilter2)
	}
}

func TestLatest(t *testing.T) {
	wantVersion := "1.2.4"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	paygen, err := parsePaygen(ctx, "testdata/TestPaygen.json")
	if err != nil {
		t.Error("Failed to load the testfile")
	}

	filtered := paygen.FilterBoardChannelDeltaType("sarien", "canary", "OMAHA")
	if latest, err := filtered.FindLatest(); err != nil {
		t.Fatal("Unexpected error in finding the latest release: ", err)
	} else if latest.ChromeOSVersion != wantVersion {
		t.Errorf("Unexpected version for the latest release; got %s, want %s", latest.ChromeOSVersion, wantVersion)
		latestJSON, err := json.MarshalIndent(latest, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Latest:\n%s", string(latestJSON))
	}
}

func TestVersion(t *testing.T) {
	major, minor, patch, err := version("1.2.4")
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if major != 1 || minor != 2 || patch != 4 {
		t.Errorf("Unexpected return values; got (%d, %d, %d), want (1, 2, 4)", major, minor, patch)
	}

	_, _, _, err = version("")
	if err == nil {
		t.Error("Unexpected result, error should not be nil")
	}
}
