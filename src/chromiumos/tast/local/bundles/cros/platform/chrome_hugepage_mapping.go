// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/process"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeHugepageMapping,
		Desc:         "Checks that hugepage of Chrome is working properly",
		Contacts:     []string{"tcwang@chromium.org"},
		SoftwareDeps: []string{"chrome", "transparent_hugepage"},
	})
}

func ChromeHugepageMapping(ctx context.Context, s *testing.State) {
	// For this test to work, some form of Chrome needs to be up and
	// running. Importantly, we must've forked zygote.
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure that our UI is running: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		CheckChromeHugepage(s)
		return nil
	}, &testing.PollOptions{
		Interval: 250 * time.Millisecond,
		Timeout:  30 * time.Second,
	}); err != nil {
		s.Error("Checking processes failed: ", err)
	}

}

func CheckChromeHugepage(s *testing.State) bool {
	hugepage_size := uint64(2 * 1024 * 1024)

	// Find out the markers information first
	marker_start, marker_end := FindOrderfileMarkers(s)
	if marker_end-marker_start >= 16*hugepage_size {
		errors.Errorf("The maximum number of hugepages we should use is 16, orderfile should contain symbols less than it")
		return false
	}
	// Find out the actual hugepage mapping
	text_start, hugepage_start, hugepage_end := FindHugepageMappings(s)
	// Find out architecture
	is_x86 := CheckIsX86(s)

	// It is important to make sure what we want actually land on hugepages
	marker_start = text_start + marker_start
	marker_end = text_start + marker_end
	if is_x86 {
		// For x86, it's ok that the first few symbols not land on hugepage because of alignment
		if marker_start%hugepage_size != 0 {
			if (marker_start/hugepage_size+1)*hugepage_size != hugepage_start {
				errors.Errorf("The begin marker is not 2MB-aligned but the next 2MB-aligned address should be on hugepage.")
				return false
			}
		} else {
			if marker_start != hugepage_start {
				errors.Errorf("The begin marker is 2MB-aligned so it should land on hugepages")
				return false
			}
		}
		if marker_end > hugepage_end {
			errors.Errorf("Symbols towards the end marker should be on hugepages")

		}
	} else {
		// For other architectures, all the orderfiles should land on hugepages
		if (marker_end > hugepage_end) || (marker_start < hugepage_start) {
			errors.Errorf("All symbols between the markers should land on hugepages")
		}
	}
	return true
}

func CheckIsX86(s *testing.State) bool {
	u, err := sysutil.Uname()
	if err != nil {
		s.Fatal("Failed to get uname() from machine.")
	}
	return strings(u.Machine, "x86")
}

func FindOrderfileMarkers(s *testing.State) (uint64, uint64) {
	contents, err := ioutil.ReadFile("/usr/local/chrome_marker_info.txt")
	if err != nil {
		if os.IsNotExist(err) {
			s.Fatal("Marker information is not on the machine.")
		}
	}
	markersRegexp := regexp.MustCompile(`\d+:\s+([a-f\d]+)\s+\d+\s+FUNC\s+LOCAL\s+DEFAULT\s+\d+\s+chrome_(begin|end)_ordered_code`)
	subm := markersRegexp.FindAllSubmatch(contents, -1)
	if (subm == nil) || (len(subm) < 2) {
		s.Error("The markers are not found")
	}
	marker_start, _ := strconv.ParseUint(string(subm[0][1]), 16, 64)
	marker_end, _ := strconv.ParseUint(string(subm[1][1]), 16, 64)
	return marker_start, marker_end
}

func GetZygotePID(s *testing.State) uint {
	procs, err := process.Processes()
	if err != nil {
		s.Fatal("Failed getting processes")
	}

	for _, proc := range procs {
		cmdline, err := proc.CmdlineSlice()
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			s.Fatal(fmt.Sprintf("failed to get cmdline for %d", proc.Pid))
		}

		// Until https://crbug.com/887875 is fixed, we need to double-split. CmdlineSlice
		// will handle splitting on \0s; we need to split on spaces.
		if len(cmdline) == 0 {
			continue
		}

		cmdline_split := strings.Fields(cmdline[0])
		if len(cmdline_split) == 0 ||
			!strings.HasSuffix(cmdline_split[0], "/chrome") ||
			!strings.Contains(cmdline[0], "--type=zygote") {
			continue
		}

		s.Log("Found Process ID of Chrome zygote at ", proc.Pid)
		return uint(proc.Pid)
	}

	s.Fatal("Chrome Zygote process is not found")
	return 0
}

func FindHugepageMappings(s *testing.State) (uint64, uint64, uint64) {
	pid := GetZygotePID(s)
	mappings, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		s.Fatal(fmt.Sprintf("Failed to read /proc/%d/maps", pid))
	}
	// Get text section start from first executable Chrome mapping
	text_start := GetTextSectionStart(s, mappings)
	// Get mappings of hugepages
	hugepage_start, hugepage_end := GetHugepageMappings(s, mappings)
	// Check hugepage mapping is sane and size is normal
	if (hugepage_start < text_start) || (hugepage_end <= hugepage_start) {
		s.Fatal("Hugepage mapping is not sane.")
	}
	hugepage_size := uint64(2 * 1024 * 1024)
	if (hugepage_start%hugepage_size != 0) || (hugepage_end%hugepage_size != 0) {
		s.Error("Hugepage mapping is not 2MB-aligned.")
	}
	hugepage_number := (hugepage_end - hugepage_start) / hugepage_size
	s.Log("Found ", hugepage_number, " huge page(s) for Chrome.")
	if (hugepage_number > 16) || (hugepage_number < 8) {
		s.Error("Hugepage number is too large (>16) or too small (<8)")
	}
	return text_start, hugepage_start, hugepage_end
}

func GetTextSectionStart(s *testing.State, mappings []byte) uint64 {
	// Find the first mappings of /opt/google/chrome/chrome
	// It should look like this:
	// 5adbd923f000-5adbd9400000 r--p 01070000 b3:03 65976 /opt/google/chrome/chrome
	// The first address is the starting address of text section
	firstChromeMappingRegexp := regexp.MustCompile(`([\da-f]+)\-[\da-f]+\s+r--p\s+[\w\s:]+/opt/google/chrome/chrome`)
	subm := firstChromeMappingRegexp.FindSubmatch(mappings)
	if subm == nil {
		s.Fatal("No chrome mappings found")
	}

	start_address, _ := strconv.ParseUint(string(subm[1]), 16, 64)
	return start_address
}

func GetHugepageMappings(s *testing.State, mappings []byte) (uint64, uint64) {
	// Find the hugepage mappings of chrome
	// Since we are using transparent hugepages, it shows on the mapping
	// like this:
	// 5ad686600000-5ad687600000 r-xp 00000000 00:00 0
	//
	// Also, to make sure the transparent hugepage mapping is for Chrome,
	// we need to make sure it is:
	// 1) After an executable Chrome mapping (optional)
	// 2) Before an executable Chrome mapping (required, since we never
	//    map the last part of text section onto hugepages)
	hugepageMappingRegexp := regexp.MustCompile(`(?:/opt/google/chrome/chrome\s+)?([\da-f]+)\-([\da-f]+) r-xp 00000000 00:00 0\s+[\w:\s\-]+/opt/google/chrome/chrome`)
	subm := hugepageMappingRegexp.FindSubmatch(mappings)
	if subm == nil {
		s.Fatal("No chrome hugepage mappings found")
	}

	start_address, _ := strconv.ParseUint(string(subm[1]), 16, 64)
	end_address, _ := strconv.ParseUint(string(subm[2]), 16, 64)
	return start_address, end_address
}
