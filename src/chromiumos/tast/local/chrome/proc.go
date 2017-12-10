// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
)

const (
	chromeExecPath = "/opt/google/chrome/chrome" // path of Chrome executable

	ppidField = "PPid:" // field name in proc status containing parent PID
)

// getParentPID returns pid's parent process's PID.
// TODO(derat): Consider instead pulling in some third-party package for parsing proc.
func getParentPID(pid int) (int, error) {
	p := fmt.Sprintf("/proc/%d/status", pid)
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return -1, err
	}
	sc := bufio.NewScanner(bytes.NewBuffer(b))
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) == 2 && parts[0] == ppidField {
			return strconv.Atoi(parts[1])
		}
	}
	return -1, fmt.Errorf("failed to find %s field in %s", ppidField, p)
}

// GetRootPID returns the PID of the root Chrome process.
// This corresponds to the browser process.
func GetRootPID(ctx context.Context) (int, error) {
	out, err := exec.Command("pidof", chromeExecPath).Output()
	if err != nil {
		return -1, err
	}

	pids := make(map[int]struct{})
	for _, ps := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(ps)
		if err != nil {
			return -1, fmt.Errorf("bad PID %q", ps)
		}
		pids[pid] = struct{}{}
	}

	for pid := range pids {
		ppid, err := getParentPID(pid)
		if err != nil {
			// Ignore errors; non-root processes can exit mid-run.
			continue
		}
		if _, ok := pids[ppid]; !ok {
			return pid, nil
		}
	}
	return -1, fmt.Errorf("root not found")
}
