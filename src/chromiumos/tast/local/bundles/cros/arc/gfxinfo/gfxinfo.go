// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gfxinfo contains common utilities for using dumpsys gfxinfo in tast-tests.
package gfxinfo

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
)

// GfxInfo stores information of dumpsys gfxinfo for a single process.
type GfxInfo struct {
	pid  uint64
	name string

	uptime   uint64
	realtime uint64

	statsSince time.Duration

	totalFramesRendered   uint64
	jankyFrames           uint64
	jankyFramesPercentage float64

	percentile50 time.Duration
	percentile90 time.Duration
	percentile95 time.Duration
	percentile99 time.Duration

	numMissedVsync           uint64
	numHighLatency           uint64
	numSlowUIThread          uint64
	numSlowBitmapUploads     uint64
	numSlowIssueDrawCommands uint64
	numFrameDeadlineMissed   uint64

	histogram map[time.Duration]uint64
}

// ResetGfxInfo resets gfxinfo for a single process.
func ResetGfxInfo(ctx context.Context, a *arc.ARC, name string) error {
	cmd := a.Command(ctx, "dumpsys", "gfxinfo", name, "reset")
	return cmd.Run()
}

// GetGfxInfo gets gfxinfo for a single process.
func GetGfxInfo(ctx context.Context, a *arc.ARC, name string) (*GfxInfo, error) {
	cmd := a.Command(ctx, "dumpsys", "gfxinfo", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	return parseGfxInfo(lines)
}

type lineParser func(*GfxInfo, string) error

func parseGfxInfo(lines []string) (*GfxInfo, error) {
	info := new(GfxInfo)
	// Prepare parsers.
	parsers := []lineParser{
		newExactLineParser("Applications Graphics Acceleration Info:"),
		uptimeParser,
		newExactLineParser(""),
		processParser,
		newExactLineParser(""),
		newDurationLineParser("Stats since", &info.statsSince),
		newUintLineParser("Total frames rendered", &info.totalFramesRendered),
		jankyFramesParser,
		newDurationLineParser("50th percentile", &info.percentile50),
		newDurationLineParser("90th percentile", &info.percentile90),
		newDurationLineParser("95th percentile", &info.percentile95),
		newDurationLineParser("99th percentile", &info.percentile99),
		newUintLineParser("Number Missed Vsync", &info.numMissedVsync),
		newUintLineParser("Number High input latency", &info.numHighLatency),
		newUintLineParser("Number Slow UI thread", &info.numSlowUIThread),
		newUintLineParser("Number Slow bitmap uploads", &info.numSlowBitmapUploads),
		newUintLineParser("Number Slow issue draw commands", &info.numSlowIssueDrawCommands),
		newUintLineParser("Number Frame deadline missed", &info.numFrameDeadlineMissed),
		histogramParser,
	}
	// Run parsers.
	for i, parser := range parsers {
		if err := parser(info, lines[i]); err != nil {
			return nil, err
		}
	}
	return info, nil
}

func newExactLineParser(expected string) lineParser {
	return func(info *GfxInfo, line string) error {
		if line != expected {
			return errors.Errorf("invalid line format, expected: %q, got: %q", expected, line)
		}
		return nil
	}
}

func newDurationLineParser(expected string, ret *time.Duration) lineParser {
	return func(info *GfxInfo, line string) error {
		parts := strings.Split(line, ": ")
		if len(parts) != 2 || parts[0] != expected {
			return errors.Errorf("invalid line format, expected: \"%s: <time.Duration>\", got: %q", expected, line)
		}
		duration, err := time.ParseDuration(parts[1])
		if err != nil {
			return errors.Errorf("error parsing duration in %q: %s", line, err.Error())
		}
		*ret = duration
		return nil
	}
}

func newUintLineParser(expected string, ret *uint64) lineParser {
	return func(info *GfxInfo, line string) error {
		parts := strings.Split(line, ": ")
		if len(parts) != 2 || parts[0] != expected {
			return errors.Errorf("invalid line format, expected: \"%s: <time.Duration>\", got: %q", expected, line)
		}
		value, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return errors.Errorf("error parsing uint64 in %q: %s", line, err.Error())
		}
		*ret = value
		return nil
	}
}

func uptimeParser(info *GfxInfo, line string) error {
	_, err := fmt.Sscanf(line, "Uptime: %d Realtime: %d", &info.uptime, &info.realtime)
	return err
}

func processParser(info *GfxInfo, line string) error {
	re := regexp.MustCompile(`^\*\* Graphics info for pid (\d+) \[(\S+)\] \*\*$`)
	match := re.FindStringSubmatch(line)
	if len(match) != 3 {
		return errors.Errorf("invalid line format, expected: \"** Graphics info for pid <pid> [<name>] **\", got: %q", line)
	}
	pid, err := strconv.ParseUint(match[1], 10, 64)
	if err != nil {
		return errors.Errorf("error parsing pid in %q: %s", line, err.Error())
	}
	info.pid = pid
	info.name = match[2]
	return nil
}

func jankyFramesParser(info *GfxInfo, line string) error {
	_, err := fmt.Sscanf(line, "Janky frames: %d (%f%%)", &info.jankyFrames, &info.jankyFramesPercentage)
	return err
}

func histogramParser(info *GfxInfo, line string) error {
	const prefix = "HISTOGRAM: "
	if !strings.HasPrefix(line, prefix) {
		return errors.Errorf("invalid line format, expected: \"HISTOGRAM: (<key>:<value>)+\", got: %q", line)
	}

	info.histogram = make(map[time.Duration]uint64)
	fields := strings.Split(line[len(prefix):], " ")
	for _, field := range fields {
		parts := strings.Split(field, "=")
		if len(parts) != 2 {
			return errors.Errorf("invalid histogram value, expected: \"<duration>:<count>\", got: %q", field)
		}
		duration, err := time.ParseDuration(parts[0])
		if err != nil {
			return errors.Errorf("error parsing duration in %q: %s", field, err.Error())
		}
		count, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return errors.Errorf("error parsing uint64 in %q: %s", field, err.Error())
		}
		info.histogram[duration] = count
	}
	return nil
}
