// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

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

var rx = regexp.MustCompile(`msg=audit\([0-9]*\.[0-9]*`)
var ry = regexp.MustCompile(`btime [0-9]*`)

// GetAuditLogsSinceBoot returns all audit logs since last boot.
func GetAuditLogsSinceBoot(ctx context.Context) (string, error) {

	statLogs, err := ioutil.ReadFile(statLogPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read status logs")
	}
	tout := string(ry.Find([]byte(statLogs)))
	tout = strings.TrimLeft(tout, "btime ")
	toutint, _ := strconv.Atoi(tout)
	bootTime := time.Unix(int64(toutint), int64(0))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse boot time")
	}
	auditLogs, err := ioutil.ReadFile(auditLogPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read audit log")
	}

	lines := strings.Split(string(auditLogs), "\n")
	for i, l := range lines {
		regMatch := string(rx.Find([]byte(l)))
		unixTime := strings.TrimLeft(regMatch, "msg=audit(")
		splitUnixTime := strings.Split(unixTime, ".")
		secnds, err := strconv.Atoi(splitUnixTime[0])
		nanosecnds, err := strconv.Atoi(splitUnixTime[1])
		if err != nil {
			return "", errors.Wrap(err, "error parsing or converting audit timestamp")
		}
		t := time.Unix(int64(secnds), int64(nanosecnds))

		if bootTime.Before(t) {
			recentLogs := strings.Join(lines[i:], "\n")
			return recentLogs, nil
		}
	}
	return "", nil
}

// GetUpstartLogsSinceBoot returns all upstart logs since last boot.
func GetUpstartLogsSinceBoot(ctx context.Context) (string, error) {
	statLogs, err := ioutil.ReadFile(statLogPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read status logs")
	}
	tout := string(ry.Find([]byte(statLogs)))
	tout = strings.TrimLeft(tout, "btime ")
	toutint, _ := strconv.Atoi(tout)
	bootTime := time.Unix(int64(toutint), int64(0))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse boot time")
	}

	upstartLogs, err := ioutil.ReadFile(upstartLogPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read upstart log")
	}
	lines := strings.Split(string(upstartLogs), "\n")
	for i, l := range lines {
		splitstr := strings.Split(l, " ")
		t, err := time.Parse(time.RFC3339, splitstr[0])
		if err != nil {
			return "", errors.Wrap(err, "failed to parse upstart timestamp")
		}
		if bootTime.Before(t) {
			recentLogs := strings.Join(lines[i:], "\n")
			return recentLogs, nil
		}
	}
	return "", nil
}
