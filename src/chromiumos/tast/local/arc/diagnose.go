// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/syslog"
)

// crashRegexp matches a line in logcat about a crashed process.
//
//  11-14 07:15:39.753   703   703 F DEBUG   : pid: 82, tid: 128, name: Binder:82_1  >>> /system/bin/mediaserver <<<
var crashRegexp = regexp.MustCompile(` F DEBUG .*>>> (.*) <<<`)

// diagnose diagnoses Android boot failure with logcat. observedErr is an error
// observed earlier, possibly not directly related to the root cause.
func diagnose(logcatPath string, observedErr error) error {
	f, err := os.Open(logcatPath)
	if err != nil {
		// This must not happen usually; diagnose is used only after successful launch of logcat.
		return errors.Wrap(observedErr, "Android failed to boot (logcat unavailable)")
	}
	defer f.Close()

	if fi, err := f.Stat(); err == nil && fi.Size() == 0 {
		return errors.Wrap(observedErr, "Android failed to boot (logcat is empty)")
	}

	// Scrape the crashed processes.
	crashed := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "*** FATAL EXCEPTION IN SYSTEM PROCESS") {
			crashed = "system_server"
			break
		}
		m := crashRegexp.FindStringSubmatch(line)
		if len(m) > 0 {
			crashed = filepath.Base(m[1])
			break
		}
	}

	if crashed != "" {
		return errors.Wrapf(observedErr, "Android failed to boot (%s crashed)", crashed)
	}
	return errors.Wrap(observedErr, "Android failed to boot")
}

// diagnoseSyslog scans for typical failures in syslog. Useful in
// diagnosing ARCVM boot failures.  Add a message if possible.
func diagnoseSyslog(ctx context.Context, reader *syslog.Reader) error {
	const crosvmInitialization = "The architecture failed to build the vm:"

	for {
		e, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errors.Wrap(err, "Trying to diagnose syslog")
		}
		if strings.Contains(e.Content, crosvmInitialization) {
			return errors.Errorf("crosvm failed initialization: %v", e.Content)
		}
	}
}
