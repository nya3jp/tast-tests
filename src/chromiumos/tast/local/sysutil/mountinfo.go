// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sysutil

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// MountOpt is a bit flag for the mount option.
type MountOpt uint32

const (
	// MntReadonly represents "ro".
	MntReadonly MountOpt = 1 << iota
	// MntNosuid represents "nosuid".
	MntNosuid
	// MntNodev represents "nodev".
	MntNodev
	// MntNoexec represents "noexec".
	MntNoexec
	// MntNoatime represents "noatime".
	MntNoatime
	// MntNodiratime represents "nodiratime".
	MntNodiratime
	// MntRelatime represents "relatime".
	MntRelatime
)

// Map from string representation in /proc/${PID}/mountinfo to a bit flag.
var optMap = map[string]MountOpt{
	// "rw" is valid mount option, but no bit flag will be set.
	// If the flag does not contain MntReadonly, it is writable.
	"rw":         0,
	"ro":         MntReadonly,
	"nosuid":     MntNosuid,
	"nodev":      MntNodev,
	"noexec":     MntNoexec,
	"noatime":    MntNoatime,
	"nodiratime": MntNodiratime,
	"relatime":   MntRelatime,
}

// MountInfo is a struct containing mount point info.
type MountInfo struct {
	MountID       int
	ParentID      int
	Major         int
	Minor         int
	Root          string
	MountPath     string
	MountOpts     MountOpt
	Shared        int // 0 if not shared
	Master        int // 0 if not a slave mount
	PropagateFrom int // 0 if propagated_from is unavailable.
	Unbindable    bool
	Fstype        string
	MountSource   string
	SuperOpts     []string
}

// lineRe is the regex to be matched with a line entry in /proc/${PID}/mountinfo.
var lineRe = regexp.MustCompile(
	"^(\\d+) (\\d+) (\\d+):(\\d+) (\\S+) (\\S+) (\\S+)(?: shared:(\\d+))?(?: master:(\\d+))?(?: propagate_from:(\\d+))?(?: (unbindable))? - (\\S+) (\\S+) (\\S+)$")

// String components has escaped characters for ' ', LF, Tab and '\'.
var unescapeRe = regexp.MustCompile("\\\\040|\\\\011|\\\\012|\\\\134")

func unescape(s string) (string, error) {
	var errs []error
	val := unescapeRe.ReplaceAllStringFunc(s, func(c string) string {
		u, err := strconv.Unquote(c)
		if err != nil {
			errs = append(errs, err)
			return c
		}
		return u
	})
	if errs != nil {
		return "", errors.Errorf("Failed to unescape %q: %v", s, errs)
	}
	return val, nil
}

// parseLine parses an entry in /proc/${PID}/mountinfo.
// Please see also "man proc" and show_mountinfo() in fs/proc_namespace.c for
// the format details.
func parseLine(line string) (MountInfo, error) {
	matches := lineRe.FindStringSubmatch(line)
	if matches == nil {
		return MountInfo{}, errors.New("unknown format: " + line)
	}

	mountID, err := strconv.Atoi(matches[1])
	if err != nil {
		return MountInfo{}, errors.Wrap(err, "failed to parse mount_id")
	}
	parentID, err := strconv.Atoi(matches[2])
	if err != nil {
		return MountInfo{}, errors.Wrap(err, "failed to parse parent_id")
	}
	major, err := strconv.Atoi(matches[3])
	if err != nil {
		return MountInfo{}, errors.Wrap(err, "failed to parse major")
	}
	minor, err := strconv.Atoi(matches[4])
	if err != nil {
		return MountInfo{}, errors.Wrap(err, "failed to parse minor")
	}
	root, err := unescape(matches[5])
	if err != nil {
		return MountInfo{}, errors.Wrap(err, "failed to parse root")
	}
	mountPath, err := unescape(matches[6])
	if err != nil {
		return MountInfo{}, errors.Wrap(err, "failed to parse mount path")
	}

	var mountOpts MountOpt
	for _, token := range strings.Split(matches[7], ",") {
		val, ok := optMap[token]
		if !ok {
			return MountInfo{}, errors.New("unknwon opt token: " + token)
		}
		mountOpts |= val
	}

	shared := 0
	if matches[8] != "" {
		shared, err = strconv.Atoi(matches[8])
		if err != nil {
			return MountInfo{}, errors.Wrap(err, "failed to parse shared")
		}
	}

	master := 0
	if matches[9] != "" {
		master, err = strconv.Atoi(matches[9])
		if err != nil {
			return MountInfo{}, errors.Wrap(err, "failed to parse master")
		}
	}

	propagated := 0
	if matches[10] != "" {
		propagated, err = strconv.Atoi(matches[10])
		if err != nil {
			return MountInfo{}, errors.Wrap(err, "failed to parse propagated_from")
		}
	}

	unbindable := matches[11] == "unbindable"
	fstype, err := unescape(matches[12])
	if err != nil {
		return MountInfo{}, errors.Wrap(err, "failed to parse fstype")
	}
	mountSource, err := unescape(matches[13])
	if err != nil {
		return MountInfo{}, errors.Wrap(err, "failed to parse mount source")
	}
	superOpts := strings.Split(matches[14], ",")

	return MountInfo{
		mountID, parentID, major, minor, root, mountPath, mountOpts,
		shared, master, propagated, unbindable, fstype, mountSource,
		superOpts}, nil
}

const (
	// SelfPID can be used as an argument of MountInfoForPID to return
	// the result for the current process.
	SelfPID = 0
)

// MountInfoForPID reads and parses the /proc/${PID}/mountinfo, and returns
// an array of mount point info for the given process.
// pid needs to be a valid PID or SelfPID.
func MountInfoForPID(pid int) ([]MountInfo, error) {
	if pid == 0 {
		pid = os.Getpid()
	}
	path := fmt.Sprintf("/proc/%d/mountinfo", pid)
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open: "+path)
	}
	defer f.Close()

	var result []MountInfo
	s := bufio.NewScanner(f)
	for s.Scan() {
		info, err := parseLine(s.Text())
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	if err := s.Err(); err != nil {
		return nil, errors.Wrap(err, "Failed to scan: "+path)
	}
	return result, nil
}
