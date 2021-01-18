// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Characteristics,
		Desc: "Verifies the format of camera characteristics file for USB cameras",
		Contacts: []string{
			"kamesan@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{caps.BuiltinUSBCamera},
	})
}

func Characteristics(ctx context.Context, s *testing.State) {
	f, err := os.Open("/etc/camera/camera_characteristics.conf")
	if err != nil {
		s.Fatal("Failed to open camera characteristics file: ", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	validators := map[string]func(string) bool{
		"constant_framerate_unsupported":    enum("true", "false"),
		"frames_to_skip_after_streamon":     nonNegInt,
		"horizontal_view_angle_16_9":        nonNegFloat,
		"horizontal_view_angle_4_3":         nonNegFloat,
		"lens_facing":                       enum("0", "1"),
		"lens_info_available_apertures":     commaSeparated(nonNegFloat),
		"lens_info_available_focal_lengths": commaSeparated(nonNegFloat),
		"lens_info_minimum_focus_distance":  nonNegFloat,
		"lens_info_optimal_focus_distance":  nonNegFloat,
		"quirks":                            commaSeparated(enum("monocole", "prefer_mjpeg", "report_least_fps_range", "v1device")),
		"sensor_info_active_array_size":     intRect,
		"sensor_info_physical_size":         floatSize,
		"sensor_info_pixel_array_size":      intSize,
		"sensor_orientation":                enum("0", "90", "180", "270"),
		"usb_vid_pid":                       uniqueVidPid(),
		"vertical_view_angle_16_9":          nonNegFloat,
		"vertical_view_angle_4_3":           nonNegFloat,
		// TODO(kamesan): Deprecate this when all the characteristics files are cleaned up.
		"resolution_1280x960_unsupported": enum("true", "false"),
	}
	lineRe := regexp.MustCompile(`^camera(\d+)(?:\.module(\d+))?\.([^=]+)=(.+)$`)
	lineNum := 0
	prevCid := -1
	prevMid := -1

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		if line == "" || line[0] == '#' {
			continue
		}
		match := lineRe.FindStringSubmatch(line)
		if len(match) != 5 {
			s.Fatal("Failed to parse line ", lineNum)
		}

		cid, err := strconv.Atoi(match[1])
		if err != nil {
			s.Fatal("Failed to parse camera id at line ", lineNum)
		}
		if cid != prevCid {
			if cid != prevCid+1 {
				s.Fatalf("Invalid camera id %v at line %v", cid, lineNum)
			}
			prevCid = cid
			prevMid = -1
		}

		if match[2] == "" {
			if prevMid != -1 {
				s.Fatal("Camera specific info comes after module specific ones at line ", lineNum)
			}
		} else {
			mid, err := strconv.Atoi(match[2])
			if err != nil {
				s.Fatal("Failed to parse module id at line ", lineNum)
			}
			if mid != prevMid {
				if mid != prevMid+1 {
					s.Fatalf("Invalid module id %v at line %v", mid, lineNum)
				}
				prevMid = mid
			}
		}

		if validator, ok := validators[match[3]]; !ok {
			s.Fatalf("Unknown property %v at line %v", match[3], lineNum)
		} else if !validator(match[4]) {
			s.Fatalf("Invalid value %v at line %v", match[4], lineNum)
		}
	}
}

func uniqueVidPid() func(string) bool {
	ids := map[string]struct{}{}
	return func(s string) bool {
		re := regexp.MustCompile(`^[0-9A-Fa-f]{4}:[0-9A-Fa-f]{4}$`)
		if !re.MatchString(s) {
			return false
		}
		s = strings.ToLower(s)
		if _, ok := ids[s]; ok {
			return false
		}
		ids[s] = struct{}{}
		return true
	}
}

func enum(items ...string) func(string) bool {
	return func(s string) bool {
		for _, item := range items {
			if s == item {
				return true
			}
		}
		return false
	}
}

func commaSeparated(isItemValid func(string) bool) func(string) bool {
	return func(s string) bool {
		re := regexp.MustCompile(`(?:^|,)([^,]+)`)
		match := re.FindAllStringSubmatch(s, -1)
		for _, m := range match {
			if !isItemValid(m[1]) {
				return false
			}
		}
		return true
	}
}

func nonNegInt(s string) bool {
	v, err := strconv.Atoi(s)
	return err == nil && v >= 0
}

func nonNegFloat(s string) bool {
	v, err := strconv.ParseFloat(s, 64)
	return err == nil && v >= 0.0
}

func intSize(s string) bool {
	re := regexp.MustCompile(`^(\d+)x(\d+)$`)
	match := re.FindStringSubmatch(s)
	if len(match) != 3 {
		return false
	}
	for _, m := range match[1:] {
		v, err := strconv.Atoi(m)
		if err != nil || v < 0 {
			return false
		}
	}
	return true
}

func floatSize(s string) bool {
	re := regexp.MustCompile(`^([0-9.]+)x([0-9.]+)$`)
	match := re.FindStringSubmatch(s)
	if len(match) != 3 {
		return false
	}
	for _, m := range match[1:] {
		v, err := strconv.ParseFloat(m, 64)
		if err != nil || v < 0.0 {
			return false
		}
	}
	return true
}

func intRect(s string) bool {
	re := regexp.MustCompile(`^(\d+),(\d+),(\d+),(\d+)$`)
	match := re.FindStringSubmatch(s)
	if len(match) != 5 {
		return false
	}
	value := [4]int{}
	for i, m := range match[1:] {
		v, err := strconv.Atoi(m)
		if err != nil || v < 0 {
			return false
		}
		value[i] = v
	}
	return value[0] < value[2] && value[1] < value[3]
}
