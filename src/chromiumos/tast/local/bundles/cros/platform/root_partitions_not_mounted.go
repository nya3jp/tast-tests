// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RootPartitionsNotMounted,
		Desc: "Check that root partitions are only mounted by processes other than  update-engine",
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
		s.Fatal("Failed to get root device: ", err)
	}
	s.Log("root device: ", device)

	parts, err := devicePartitions(ctx, device)
	if err != nil {
		s.Fatal("Failed to get device partitions: ", err)
	}
	s.Log("root device partitions: ", parts)

	prs, err := processList()
	if err != nil {
		s.Fatal("Failed to get process list: ", err)
	}
	for _, pr := range prs {
		mount := filepath.Join("/proc", strconv.FormatUint(uint64(pr.Pid), 10), "mounts")
		devices, err := mountedDevices(mount)
		if err != nil {
			s.Fatal("Failed to get mounted devices: ", err)
		}
		for _, p := range parts {
			for _, n := range devices {
				if p != n {
					continue
				}
				name, _ := pr.Name()
				s.Errorf("Root partition %s is mounted by process %s (%d)", p, name, pr.Pid)
			}
		}
	}
}

func rootDevice(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "/usr/bin/rootdev", "-s", "-d").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func devicePartitions(ctx context.Context, device string) ([]string, error) {
	out, err := testexec.CommandContext(ctx, "/usr/bin/cgpt", "find", "-t", "rootfs", device).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n"), nil
}

func mountedDevices(mount string) ([]string, error) {
	file, err := os.Open(mount)
	if err != nil {
		return nil, errors.Wrap(err, "fail to open mount file")
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var devices []string
	for scanner.Scan() {
		if !strings.HasPrefix(scanner.Text(), "/dev/") {
			continue
		}
		devices = append(devices, strings.Split(scanner.Text(), " ")[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "fail to scan mount file")
	}

	return devices, nil
}

func processList() ([]*process.Process, error) {
	infos, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "fail to get list of processes")
	}
	var processes []*process.Process
	for _, pr := range infos {
		// Ignore update_engine since the process mounts root partitions.
		if name, err := pr.Exe(); err == nil && name != "/usr/sbin/update_engine" {
			processes = append(processes, pr)
		}
	}
	return processes, nil
}
