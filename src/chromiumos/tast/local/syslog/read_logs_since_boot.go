// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syslog

import (
	"context"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

const (
	auditLogPath   = "/var/log/audit/audit.log"
	upstartLogPath = "/var/log/upstart.log"
	statLogPath    = "/proc/stat"
)

// GetBootTime returns boot time as a Time.
func GetBootTime() (time.Time, error) {
	statLogs, err := ioutil.ReadFile(statLogPath)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to read status logs")
	}
	rx := regexp.MustCompile(`(?:btime )([0-9]*)`)
	tout := rx.FindStringSubmatch(string(statLogs))
	toutint, err := strconv.ParseInt(tout[1], 10, 64)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "error parsing boot time")
	}
	bootTime := time.Unix(toutint, 0)
	return bootTime, nil
}

// GetAuditLogsSinceBoot returns all audit logs since last boot.
func GetAuditLogsSinceBoot(ctx context.Context) ([]string, error) {
	bootTime, err := GetBootTime()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get boot time")
	}
	auditLogs, err := ioutil.ReadFile(auditLogPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read audit log")
	}
	rx := regexp.MustCompile(`(?:msg=audit\()([0-9]*)\.([0-9]*)`)
	lines := strings.Split(string(auditLogs), "\n")
	for i, l := range lines {
		regMatch := rx.FindStringSubmatch(l)
		if len(regMatch) < 2 {
			continue
		}
		seconds, err := strconv.ParseInt(regMatch[1], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "error parsing audit timestamp")
		}
		nanoseconds, err := strconv.ParseInt(regMatch[2], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "error parsing audit timestamp")
		}
		t := time.Unix(seconds, nanoseconds)

		if bootTime.Before(t) {
			recentLogs := lines[i:]
			return recentLogs, nil
		}
	}
	return nil, nil
}

// GetUpstartLogsSinceBoot returns all upstart logs since last boot.
func GetUpstartLogsSinceBoot(ctx context.Context) ([]string, error) {
	bootTime, err := GetBootTime()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get boot time")
	}

	upstartLogs, err := ioutil.ReadFile(upstartLogPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read upstart log")
	}
	lines := strings.Split(string(upstartLogs), "\n")
	for i, l := range lines {
		splitstr := strings.Split(l, " ")
		t, err := time.Parse(time.RFC3339, splitstr[0])
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse upstart timestamp")
		}
		if bootTime.Before(t) {
			recentLogs := lines[i:]
			return recentLogs, nil
		}
	}
	return nil, nil
}
