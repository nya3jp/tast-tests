// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"path/filepath"

	"github.com/shirou/gopsutil/process"
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

func RootPartitionsNotMounted(ctx context.Context, s *testing.State) {
	device, err := rootDevice(ctx)
	if err != nil {
		s.Fatalf("Failed to get root device: %q", err)
	}
	s.Log("root device: ", device)

	part, err := devicePartitions(ctx, device)
	if err != nil {
		s.Fatalf("Failed to get device partitions: %q", err)
	}
	s.Log("root device partitions:", part)

	ids, err := processIDList([]string{"/usr/sbin/update_engine"})
	if err != nil {
		s.Fatalf("Failed to get process ID list: %q", err)
	}
	for _, id := range ids {
		mount := filepath.Join("/proc/", id, "/mounts")
		devices, err := mountedDevices(mount)
		if err != nil {
			s.Fatalf("Failed to get mounted devices: %q", err)
		}
		for _, p := range part {
			for _, n := range devices {
				if p != n {
					continue
				}
				s.Errorf("Root partition %q is mounted by process %q", p, id)
			}
		}
	}
}

func rootDevice(ctx context.Context) (string, error) {
	args := []string{"/usr/bin/rootdev", "-s", "-d"}
	out, err := testexec.CommandContext(ctx, args[0], args[1:]...).Output()
	if err != nil {
		return "", errors.New(fmt.Sprintf("command: %q error: %q", strings.Join(args, " "), err))
	}
	return strings.Replace(string(out), "\n", "", -1), nil
}

func devicePartitions(ctx context.Context, device string) ([]string, error) {
	args := []string{"/usr/bin/cgpt", "find", "-t", "rootfs", device}
	out, err := testexec.CommandContext(ctx, args[0], args[1:]...).Output()
	if err != nil {
		return []string{}, errors.New(fmt.Sprintf("%q error: %q", strings.Join(args, ""), err))
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n"), err
}

func mountedDevices(mount string) ([]string, error) {
	file, err := os.Open(mount)
	if err != nil {
		return []string{}, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var devices []string
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return []string{}, err
		}
		if !strings.HasPrefix(scanner.Text(), "/dev/") {
			continue
		}
		devices = append(devices, strings.Split(strings.TrimSuffix(scanner.Text(), "\n"), " ")[0])
	}

	return devices, nil
}

func processFiltered(name string, filteredExec []string) bool {
	for _, file := range filteredExec {
		if file == name {
			return true
		}
	}
	return false
}

func processIDList(filteredExec []string) ([]string, error) {
	infos, err := process.Processes()
	if err != nil {
		return []string{}, err
	}
	var ids []string
	for _, pr := range infos {
		name, err := pr.Exe()
		if err == nil && !processFiltered(name, filteredExec) {
			ids = append(ids, strconv.FormatUint(uint64(pr.Pid), 10))
		}
	}
	return ids, nil
}
