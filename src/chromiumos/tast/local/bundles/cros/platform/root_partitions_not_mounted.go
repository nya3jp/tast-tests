// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RootPartitionsNotMounted,
		Desc: "Check that root partitions are only mounted by processes other than update-engine",
		Contacts: []string{
			"benchan@chromium.org", // Autotest author
			"puthik@chromium.org",  // Autotest author
			"chavey@chromium.org",  // Migrated autotest to tast
		},
		Attr: []string{"group:mainline"},
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
		devices, err := mountedDevices(int(pr.Pid))
		if err != nil {
			s.Logf("Failed to get mounted devices for pid %d: %v", pr.Pid, err)
		}
		for _, part := range parts {
			if _, present := devices[part]; !present {
				continue
			}
			name, err := pr.Name()
			if err != nil {
				name = "unknown"
			}
			s.Errorf("Root partition %s is mounted by process %s (%d)", part, name, pr.Pid)
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

func mountedDevices(pid int) (map[string]struct{}, error) {
	infos, err := sysutil.MountInfoForPID(pid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mount info")
	}
	var devices = make(map[string]struct{})
	for _, info := range infos {
		devices[info.MountSource] = struct{}{}
	}
	return devices, nil
}

func processList() ([]*process.Process, error) {
	infos, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get list of processes")
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
