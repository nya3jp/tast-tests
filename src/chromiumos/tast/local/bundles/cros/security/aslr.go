// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ASLR,
		Desc: "Verifies that address space is randomized between runs",
		Contacts: []string{
			"jorgelo@chromium.org",  // Security team
			"ejcaruso@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"aslr"},
		Attr:         []string{"group:mainline"},
	})
}

func ASLR(ctx context.Context, s *testing.State) {
	type deviceNumber struct {
		major uint64
		minor uint64
	}

	type addressMapping struct {
		start  uintptr
		end    uintptr
		prot   int
		shared bool
		offset uint64
		device deviceNumber
		inode  uint64
		name   string
	}

	type addressMap []addressMapping

	parseNum := func(str string, base int) uint64 {
		parsed, err := strconv.ParseUint(str, base, 64)
		if err != nil {
			// Fataling here should be fine since we already do validation
			// when we match on the regex below.
			s.Fatalf("Failed to parse %v as base %v: %v", str, base, err)
		}
		return parsed
	}

	parseAddressMap := func(reader io.Reader) addressMap {
		mapping := `([0-9a-f]+)-([0-9a-f]+) +` + // start (1) and end (2)
			`(r|-)(w|-)(x|-)(s|p) +` + // protections (3-5) and sharing (6)
			`([0-9a-f]+) +` + // offset (7)
			`([0-9a-f]+):([0-9a-f]+) +` + // device major (8) and minor (9)
			`(\d+) *` + // inode number (10)
			`(.*)` // name (11)
		mappingMatcher := regexp.MustCompile(mapping)

		var am addressMap
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			group := mappingMatcher.FindStringSubmatch(line)
			if group == nil {
				s.Fatalf("Address map file line failed to parse: %q", line)
			}

			start := uintptr(parseNum(group[1], 16))
			end := uintptr(parseNum(group[2], 16))
			prot := 0
			if group[3][0] == 'r' {
				prot |= unix.PROT_READ
			}
			if group[4][0] == 'w' {
				prot |= unix.PROT_WRITE
			}
			if group[5][0] == 'x' {
				prot |= unix.PROT_EXEC
			}
			shared := (group[6][0] == 's')
			offset := parseNum(group[7], 16)
			major := parseNum(group[8], 16)
			minor := parseNum(group[9], 16)
			inode := parseNum(group[10], 10)
			am = append(am, addressMapping{
				start, end, prot, shared, offset,
				deviceNumber{major, minor}, inode, group[11],
			})
		}

		if err := scanner.Err(); err != nil {
			s.Fatal("Failed to read map file: ", err)
		}

		return am
	}

	dumpMap := func(am addressMap, filename, header string) error {
		dumpFile, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer dumpFile.Close()

		if _, err = dumpFile.WriteString(header); err != nil {
			return err
		}
		for _, mapping := range am {
			prot := ""
			add := func(v bool, t, f string) {
				if v {
					prot += t
				} else {
					prot += f
				}
			}
			add(mapping.prot&unix.PROT_READ != 0, "r", "-")
			add(mapping.prot&unix.PROT_WRITE != 0, "w", "-")
			add(mapping.prot&unix.PROT_EXEC != 0, "x", "-")
			add(mapping.shared, "s", "p")

			line := fmt.Sprintf("%016x-%016x %v %8x %02x:%02x %-10d %v\n",
				mapping.start, mapping.end, prot, mapping.offset,
				mapping.device.major, mapping.device.minor, mapping.inode,
				mapping.name)
			if _, err = dumpFile.WriteString(line); err != nil {
				return err
			}
		}
		return nil
	}

	// Restarts job and returns memory mapping names to start addresses.
	getNewJobMap := func(job string) map[string]uintptr {
		if err := upstart.RestartJob(ctx, job); err != nil {
			s.Fatalf("Job %v did not restart: %v", job, err)
		}
		_, _, pid, err := upstart.JobStatus(ctx, job)
		if err != nil {
			s.Fatalf("Could not get status for job %v: %v", job, err)
		}

		mapFile, err := os.Open(fmt.Sprintf("/proc/%v/maps", pid))
		if err != nil {
			s.Fatalf("Could not open address map for job %v: %v", job, err)
		}
		defer mapFile.Close()
		am := parseAddressMap(mapFile)

		// dump maps to text file for future inspection if necessary
		newMapPath := filepath.Join(s.OutDir(), fmt.Sprintf("%v.txt", job))
		header := fmt.Sprintf("\n=== pid %v ===\n\n", pid)
		dumpMap(am, newMapPath, header)

		starts := make(map[string]uintptr)
		for _, mapping := range am {
			// There will probably be multiple mappings for a lot of the files mapped into
			// memory. To deal with this, we only check the mappings with offset 0.
			if (mapping.name != "[heap]" && mapping.name != "[stack]" &&
				mapping.inode == 0) || mapping.offset != 0 {
				// This isn't a mapped file or a private mapping we care about. Skip it.
				continue
			}

			starts[mapping.name] = mapping.start
		}
		return starts
	}

	const iterations = 5
	testRandomization := func(job string) {
		s.Log("Testing job ", job)
		// allStarts is a map of vmarea name to start addresses and the number of times each
		// start address has been seen across all iterations.
		type addrCounts map[uintptr]int
		allStarts := make(map[string]addrCounts)
		for name, start := range getNewJobMap(job) {
			startSet := make(map[uintptr]int)
			startSet[start] = 1
			allStarts[name] = startSet
		}

		// Collect start addresses for vm areas over several job spawns.
		for i := 0; i < iterations; i++ {
			newStarts := getNewJobMap(job)
			for name := range allStarts {
				if otherStart, present := newStarts[name]; present {
					allStarts[name][otherStart]++
				}
			}
		}

		// Check that at least one address was different for each vm area.
		for name, starts := range allStarts {
			if len(starts) == 1 {
				for start, occurrences := range starts {
					if occurrences == 1 {
						// This isn't actually a duplicate address; it only showed up once.
						continue
					}
					s.Errorf("Mapping for %v always occurred at %#x", name, start)
				}
			}
		}
	}

	for _, job := range []string{"ui", "debugd", "update-engine"} {
		testRandomization(job)
	}
}
