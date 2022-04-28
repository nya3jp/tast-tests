// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Encoder attributes parsed from modetest.
type Encoder struct {
	EncoderID      uint32
	CrtcID         uint32
	EncoderType    string
	PossibleCrtcs  uint32
	PossibleClones uint32
}

// Connector attributes parsed from modetest.
type Connector struct {
	ConnectorID uint32
	EncoderID   uint32
	Connected   bool
	Name        string
	Width       uint32
	Height      uint32
	CountModes  int
	Encoders    []uint32
	Modes       []*Mode
}

// Crtc attributes parsed from modetest.
type Crtc struct {
	CrtcID        uint32
	FrameBufferID uint32
	X             uint32
	Y             uint32
	Width         uint32
	Height        uint32
	Mode          *Mode
}

// Mode attributes parsed from modetest.
type Mode struct {
	Index      int
	Name       string
	Refresh    float64
	HDisplay   uint16
	HSyncStart uint16
	HSyncEnd   uint16
	HTotal     uint16
	VDisplay   uint16
	VSyncStart uint16
	VSyncEnd   uint16
	VTotal     uint16
}

// DumpModetestOnError dumps the output of modetest to a file if the test failed.
func DumpModetestOnError(ctx context.Context, outDir string, hasError func() bool) {
	if !hasError() {
		return
	}
	file := filepath.Join(outDir, "modetest.txt")
	f, err := os.Create(file)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to create %s: %v", file, err)
		return
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, "modetest", "-c")
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Run(); err != nil {
		testing.ContextLog(ctx, "Failed to run modetest: ", err)
	}
}

var modesetEncoderPattern = regexp.MustCompile(
	`^(\d+)\s+(\d+)\s+(\S+)\s+0x([0-9a-f]+)\s+0x([0-9a-f]+)$`)

// ModetestEncoders returns the list of encoders parsed from modetest.
func ModetestEncoders(ctx context.Context) ([]*Encoder, error) {
	output, err := testexec.CommandContext(ctx, "modetest", "-e").Output()
	if err != nil {
		return nil, err
	}

	var encoders []*Encoder
	for _, line := range strings.Split(string(output), "\n") {
		matches := modesetEncoderPattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		encoderID, err := strconv.ParseUint(matches[1], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse encoder id %s", matches[1])
		}
		crtcID, err := strconv.ParseUint(matches[2], 10, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse crtc id %s", matches[2])
		}
		encoderType := matches[3]
		possibleCrtcs, err := strconv.ParseUint(matches[4], 16, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse possible crtcs %s",
				matches[4])
		}
		possibleClones, err := strconv.ParseUint(matches[5], 16, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse possible clones %s",
				matches[5])
		}
		encoders = append(encoders, &Encoder{
			EncoderID:      uint32(encoderID),
			CrtcID:         uint32(crtcID),
			EncoderType:    encoderType,
			PossibleCrtcs:  uint32(possibleCrtcs),
			PossibleClones: uint32(possibleClones),
		})
	}
	return encoders, nil
}

var modesetConnectorPattern = regexp.MustCompile(
	`^(\d+)\s+(\d+)\s+(connected|disconnected)\s+(\S+)\s+(\d+)x(\d+)\s+(\d+)\s+(.+)$`)

// splitAndConvertInt splits string with comma and whitespace then converts each sub-string to int.
func splitAndConvertInt(input string) ([]uint32, error) {
	splitPattern := regexp.MustCompile(` *, *`)
	substrings := splitPattern.Split(input, -1)
	var result []uint32
	for _, substring := range substrings {
		i, err := strconv.ParseUint(substring, 10, 32)
		if err != nil {
			return nil, err
		}
		result = append(result, uint32(i))
	}
	return result, nil
}

// ModetestConnectors returns the list of connectors parsed from modetest.
func ModetestConnectors(ctx context.Context) ([]*Connector, error) {
	output, err := testexec.CommandContext(ctx, "modetest", "-c").Output()
	if err != nil {
		return nil, err
	}

	var connectors []*Connector
	for _, line := range strings.Split(string(output), "\n") {
		if matches := modesetConnectorPattern.FindStringSubmatch(line); matches != nil {
			connectorID, err := strconv.ParseUint(matches[1], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse connector id %s",
					matches[1])
			}
			encoderID, err := strconv.ParseUint(matches[2], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse encoder id %s",
					matches[2])
			}
			connected := (matches[3] == "connected")
			name := matches[4]
			width, err := strconv.ParseUint(matches[5], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse width %s",
					matches[5])
			}
			height, err := strconv.ParseUint(matches[6], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse height %s",
					matches[6])
			}
			countModes, err := strconv.Atoi(matches[7])
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse countModes %s",
					matches[7])
			}
			encoders, err := splitAndConvertInt(matches[8])
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse encoders %s",
					matches[8])
			}
			connectors = append(connectors, &Connector{
				ConnectorID: uint32(connectorID),
				EncoderID:   uint32(encoderID),
				Connected:   connected,
				Name:        name,
				Width:       uint32(width),
				Height:      uint32(height),
				CountModes:  countModes,
				Encoders:    encoders,
				Modes:       []*Mode{},
			})
		} else if matches := modesetModePattern.FindStringSubmatch(line); matches != nil {
			mode, err := parseMode(matches)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse mode")
			}
			lastConnector := connectors[len(connectors)-1]
			lastConnector.Modes = append(lastConnector.Modes, mode)
		}
	}
	return connectors, nil
}

// NumberOfOutputsConnected returns the number of connected connectors from modetest.
func NumberOfOutputsConnected(ctx context.Context) (int, error) {
	connectors, err := ModetestConnectors(ctx)
	if err != nil {
		return 0, err
	}
	connected := 0
	for _, display := range connectors {
		if display.Connected {
			connected++
		}
	}
	return connected, nil
}

var modesetCrtcPattern = regexp.MustCompile(`^(\d+)\s+(\d+)\s+\((\d+),(\d+)\)\s+\((\d+)x(\d+)\)$`)

// ModetestCrtcs returns the list of crtcs parsed from modetest.
func ModetestCrtcs(ctx context.Context) ([]*Crtc, error) {
	output, err := testexec.CommandContext(ctx, "modetest", "-p").Output()
	if err != nil {
		return nil, err
	}

	var crtcs []*Crtc
	for _, line := range strings.Split(string(output), "\n") {
		if matches := modesetCrtcPattern.FindStringSubmatch(line); matches != nil {
			crtcID, err := strconv.ParseUint(matches[1], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse crtc id %s",
					matches[1])
			}
			frameBufferID, err := strconv.ParseUint(matches[2], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse fb id %s",
					matches[2])
			}
			x, err := strconv.ParseUint(matches[3], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse x-position %s",
					matches[3])
			}
			y, err := strconv.ParseUint(matches[4], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse y-position %s",
					matches[4])
			}
			width, err := strconv.ParseUint(matches[5], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse width %s",
					matches[5])
			}
			height, err := strconv.ParseUint(matches[6], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse height %s",
					matches[6])
			}
			crtcs = append(crtcs, &Crtc{
				CrtcID:        uint32(crtcID),
				FrameBufferID: uint32(frameBufferID),
				X:             uint32(x),
				Y:             uint32(y),
				Width:         uint32(width),
				Height:        uint32(height),
				Mode:          nil,
			})
		} else if matches := modesetModePattern.FindStringSubmatch(line); matches != nil {
			mode, err := parseMode(matches)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse mode")
			}
			crtcs[len(crtcs)-1].Mode = mode
		}
	}
	return crtcs, nil

}

var modesetModePattern = regexp.MustCompile(
	`^\s*#(\d+)\s+(\S+)\s+(\d+\.?\d*)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)?.*$`)

// parseMode returns the mode parsed from the provided array of regexp substring matches.
func parseMode(matches []string) (*Mode, error) {
	index, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode index %s", matches[1])
	}
	name := matches[2]
	refresh, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode refresh rate %s", matches[3])
	}
	hDisplay, err := strconv.ParseUint(matches[4], 10, 16)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode hDisplay %s", matches[4])
	}
	hSyncStart, err := strconv.ParseUint(matches[5], 10, 16)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode hSyncStart %s", matches[5])
	}
	hSyncEnd, err := strconv.ParseUint(matches[6], 10, 16)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode hSyncEnd %s", matches[6])
	}
	hTotal, err := strconv.ParseUint(matches[7], 10, 16)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode hTotal %s", matches[7])
	}
	vDisplay, err := strconv.ParseUint(matches[8], 10, 16)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode vDisplay %s", matches[8])
	}
	vSyncStart, err := strconv.ParseUint(matches[9], 10, 16)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode vSyncStart %s", matches[9])
	}
	vSyncEnd, err := strconv.ParseUint(matches[10], 10, 16)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode vSyncEnd %s", matches[10])
	}
	vTotal, err := strconv.ParseUint(matches[11], 10, 16)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse mode vTotal %s", matches[11])
	}
	return &Mode{
		Index:      index,
		Name:       name,
		Refresh:    refresh,
		HDisplay:   uint16(hDisplay),
		HSyncStart: uint16(hSyncStart),
		HSyncEnd:   uint16(hSyncEnd),
		HTotal:     uint16(hTotal),
		VDisplay:   uint16(vDisplay),
		VSyncStart: uint16(vSyncStart),
		VSyncEnd:   uint16(vSyncEnd),
		VTotal:     uint16(vTotal),
	}, nil
}
