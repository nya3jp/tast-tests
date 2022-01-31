// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateutil

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// Paygen is a structure that can hold the unmarshaled paygen.json data.
type Paygen struct {
	Deltas []Delta `json:"delta"`
}

// Delta describes an update delta in the paygen.json file.
type Delta struct {
	Board             Board    `json:"board"`
	DeltaType         string   `json:"delta_type"`
	Channel           string   `json:"channel"`
	ChromeOSVersion   string   `json:"chrome_os_version"`
	ChromeVersion     string   `json:"chrome_version"`
	Milestone         int      `json:"milestone"`
	GenerateDelta     bool     `json:"generate_delta"`
	DeltaPayloadTests bool     `json:"delta_payload_tests"`
	FullPayloadTests  bool     `json:"full_payload_tests"`
	Models            []string `json:"applicable_models"`
}

// Board stores the board related data in a Delta.
type Board struct {
	PublicCodename string `json:"public_codename"`
	IsActive       bool   `json:"is_active"`
	BuilderName    string `json:"builder_name"`
}

// FindLatestStable finds the entry in paygen.json for the latest image on the selected board.
func FindLatestStable(ctx context.Context, board string) (*Delta, error) {
	channel := "stable"
	deltaType := "OMAHA"

	paygen, err := LoadPaygenFromGS(ctx)
	if err != nil {
		return nil, err
	}

	filtered := paygen.FilterBoardChannelDeltaType(board, channel, deltaType)

	return filtered.FindLatest()
}

// LoadPaygenFromGS downloads the paygen.json file, parses it, and returns it in a Paygen object.
func LoadPaygenFromGS(ctx context.Context) (*Paygen, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	src := "gs://chromeos-build-release-console/paygen.json"

	dir, err := ioutil.TempDir("", "paygen")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	defer os.RemoveAll(dir)

	if err := testexec.CommandContext(ctx, "gsutil", "copy", src, dir).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to download paygen.json")
	}

	return parsePaygen(ctx, filepath.Join(dir, "paygen.json"))
}

func parsePaygen(ctx context.Context, path string) (*Paygen, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed opening %s", path)
	}
	defer f.Close()

	var paygen Paygen
	if err := json.NewDecoder(f).Decode(&paygen); err != nil {
		return nil, errors.Wrapf(err, "failed to decode %q", path)
	}
	return &paygen, nil
}

// FilterBoardChannelDeltaType filters the deltas based on the board, channel and delta type.
// This is the most common filter combination, so it is done in one call.
func (p *Paygen) FilterBoardChannelDeltaType(board, channel, deltaType string) *Paygen {
	var filtered Paygen
	for _, delta := range p.Deltas {
		if delta.Board.PublicCodename == board && delta.Channel == channel && delta.DeltaType == deltaType {
			filtered.Deltas = append(filtered.Deltas, delta)
		}
	}

	return &filtered
}

// FilterBoard filters for a specific board string.
func (p Paygen) FilterBoard(board string) *Paygen {
	var filtered Paygen
	for _, delta := range p.Deltas {
		if delta.Board.PublicCodename == board {
			filtered.Deltas = append(filtered.Deltas, delta)
		}
	}

	return &filtered
}

// FilterDeltaType filters by type.
func (p Paygen) FilterDeltaType(deltaType string) *Paygen {
	var filtered Paygen
	for _, delta := range p.Deltas {
		if delta.DeltaType == deltaType {
			filtered.Deltas = append(filtered.Deltas, delta)
		}
	}

	return &filtered
}

// FilterMilestone filters the deltas based on the milestone.
func (p Paygen) FilterMilestone(milestone int) *Paygen {
	var filtered Paygen
	for _, delta := range p.Deltas {
		if delta.Milestone == milestone {
			filtered.Deltas = append(filtered.Deltas, delta)
		}
	}

	return &filtered
}

// FindLatest returns the Delta with the highest Chrome OS Version value.
func (p *Paygen) FindLatest() (*Delta, error) {
	if len(p.Deltas) == 0 {
		return nil, errors.New("emtpy input")
	}

	var latest *Delta
	majorMax, minorMax, patchMax := 0, 0, 0

	for _, delta := range p.Deltas {
		newer := false
		major, minor, patch, err := version(delta.ChromeOSVersion)
		if err != nil {
			continue // There are deltas without Chrome OS Version.
		}

		if major > majorMax {
			newer = true
		} else if major == majorMax {
			if minor > minorMax {
				newer = true
			} else if minor == minorMax && patch > patchMax {
				newer = true
			}
		}

		if newer {
			majorMax, minorMax, patchMax = major, minor, patch
			latest = &delta
		}
	}

	if latest == nil {
		return nil, errors.New("none of the deltas contained Chrome OS Version")
	}

	return latest, nil
}

func version(chromeOSVersion string) (int, int, int, error) {
	versionSlice := strings.Split(chromeOSVersion, ".")
	if len(versionSlice) != 3 {
		return 0, 0, 0, errors.Errorf("unexpected version format %q", chromeOSVersion)
	}

	major, err := strconv.Atoi(versionSlice[0])
	if err != nil {
		return 0, 0, 0, errors.Errorf("major version is not an integer %q", versionSlice[0])
	}

	minor, err := strconv.Atoi(versionSlice[1])
	if err != nil {
		return 0, 0, 0, errors.Errorf("minor version is not an integer %q", versionSlice[1])
	}

	patch, err := strconv.Atoi(versionSlice[2])
	if err != nil {
		return 0, 0, 0, errors.Errorf("patch number is not an integer %q", versionSlice[2])
	}

	return major, minor, patch, nil
}
