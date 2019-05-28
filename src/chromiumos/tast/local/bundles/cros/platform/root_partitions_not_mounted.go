// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bufio"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RootPartitionsNotMounted,
		Desc: "Check that root partitions are only mounted by processes update-engine",
		Contacts: []string{
			"benchan@chromium.org", // Autotest author
			"puthik@chromium.org",  // Autotest author
			"chavey@chromium.org",  // Migrated autotest to tast
		},
		Attr: []string{"informational"},
	})
}

const cgcpPath = "/usr/bin/cgpt"
const rootDevPath = "/usr/bin/rootdev"
const updateEnginePath = "/usr/sbin/update_engine"

func RootPartitionsNotMounted(ctx context.Context, s *testing.State) {
	device := getRootDevice(ctx, s)
	s.Log("root device: ", device)

	partitions := getDevicePartitions(ctx, s, device)
	s.Log("root device partitions:", partitions)

	ids := getProcessIDList(ctx, s, []string{updateEnginePath})

	for _, id := range ids {
		mount := strings.Join([]string{"/proc/", id, "/mounts"}, "")
		devices := getMountedDevices(mount)
		for _, p := range partitions {
			for _, n := range devices {
				if p != n {
					continue
				}
				s.Error("Root partition %q is mounted by process %q", p, id)
			}
		}
	}
}

func getRootDevice(ctx context.Context, s *testing.State) string {
	args := []string{rootDevPath, "-s", "-d"}
	s.Log("Get root devices:", strings.Join(args, " "))
	out, err := testexec.CommandContext(ctx, args[0], args[1:]...).Output()
	if err != nil {
		s.Fatal("Failed to run rootdev:", rootDevPath, err)
	}
	return strings.Replace(string(out), "\n", "", -1)
}

func getDevicePartitions(ctx context.Context, s *testing.State, device string) []string {
	args := []string{cgcpPath, "find", "-t", "rootfs", device}
	s.Log("Get device partitions:", strings.Join(args, " "))
	out, err := testexec.CommandContext(ctx, args[0], args[1:]...).Output()
	if err != nil {
		s.Fatal("Failed to run cgcp: ", err)
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
}

func getMountedDevices(mountsFile string) []string {
	var devices []string
	file, err := os.Open(mountsFile)
	if err != nil {
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
		}
		if !strings.HasPrefix(scanner.Text(), "/dev/") {
			continue
		}
		devices = append(devices, strings.Split(
			strings.Replace(scanner.Text(), "\n", "", -1), " ")[0])
	}

	return devices
}

func isProcessExecutable(s *testing.State, pid string, filteredExec []string) bool {
	link, err := os.Readlink(strings.Join([]string{"/proc/", pid, "/exe"}, ""))
	if err == nil {
		return true
	}
	// check if the link is in the filtered exec
	for _, file := range filteredExec {
		if file == link {
			s.Log("Filtered exec:", file)
			return false
		}
	}
	return false
}

func getProcessIDList(ctx context.Context, s *testing.State, filteredExec []string) []string {
	var ids []string
	files, err := filepath.Glob("/proc/*")
	if err != nil {
		s.Fatal("Failed to get list of process files: ", err)
	}
	for _, file := range files {
		_, name := filepath.Split(file)
		_, err := strconv.ParseInt(name, 10, 64)
		if err == nil && isProcessExecutable(s, name, filteredExec) {
			ids = append(ids, name)
		}
	}
	return ids
}
