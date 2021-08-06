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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	auditLogPath = "/var/log/audit/audit.log"
)

var rx = regexp.MustCompile(`msg=audit\([0-9]*\.[0-9]*`)

// GetAuditLogsSinceBoot returns all audit logs since last boot.
func GetAuditLogsSinceBoot(ctx context.Context) ([]string, error) {
	// Get timestamp of last boot using uptime.
	cmd := testexec.CommandContext(ctx, "uptime", "-s")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return make([]string, 0), errors.Wrap(err, "failed to get boot time")
	}
	bootTime, err := time.Parse("2006-01-02 15:04:05\n", string(out))
	if err != nil {
		return make([]string, 0), errors.Wrap(err, "failed to parse boot time")
	}
	auditLogs, err := ioutil.ReadFile(auditLogPath)
	if err != nil {
		return make([]string, 0), errors.Wrap(err, "failed to read audit log")
	}

	recentLogs := make([]string, 0)
	lines := strings.Split(string(auditLogs), "\n")
	for i, l := range lines {
		regMatch := string(rx.Find([]byte(l)))
		unixTime := strings.TrimLeft(regMatch, "msg=audit(")
		splitUnixTime := strings.Split(unixTime, ".")
		secnds, err := strconv.Atoi(splitUnixTime[0])
		nanosecnds, err := strconv.Atoi(splitUnixTime[1])
		if err != nil {
			return make([]string, 0), errors.Wrap(err, "error parsing or converting audit timestamp")
		}
		t := time.Unix(int64(secnds), int64(nanosecnds))

		if bootTime.Before(t) {
			recentLogs = lines[i:]
			break
		}
	}
	return recentLogs, nil
}
