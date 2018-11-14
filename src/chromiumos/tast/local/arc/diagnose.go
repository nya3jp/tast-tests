// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/errors"
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
		f, _ = os.Open("/dev/null")
	}
	defer f.Close()

	// Scrape the crashed processes.
	crashed := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
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
