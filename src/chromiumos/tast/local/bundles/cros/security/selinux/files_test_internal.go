// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"context"

	"chromiumos/tast/testing"
)

// FileTestCase specifies a single test case for files to test for SELinux labels
// Files should have been labeled by platform2/sepolicy/file_contexts/ or
// platform2/sepolicy/policy/*/genfs_contexts with a few exceptions.
// Exceptions include:
//  - type_transition rule to default assign a label for files created
// under some condition.
//  - mv/cp files without preserving original labels but inheriting
// labels from new parent directory (e.g. /var/log/mount-encrypted.log)
type FileTestCase struct {
	Path      string // absolute file path
	Context   string // expected SELinux file context
	Recursive bool
	Filter    FileLabelCheckFilter
	Log       bool
}

// FilesTestInternal runs the test suite for SELinuxFilesSystem(Informational)?
func FilesTestInternal(ctx context.Context, s *testing.State, testCases []FileTestCase) {
	for _, testCase := range testCases {
		filter := testCase.Filter
		if filter == nil {
			filter = CheckAll
		}
		expected, err := FileContextRegexp(testCase.Context)
		if err != nil {
			s.Errorf("Failed to compile expected context %q: %v", testCase.Context, err)
			continue
		}
		CheckContext(ctx, s, testCase.Path, expected, testCase.Recursive, filter, testCase.Log)
	}
}
