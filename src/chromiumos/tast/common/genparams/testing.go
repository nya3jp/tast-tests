// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package genparams

// TestingT is a subset of testing.T. Exported functions in this package take
// TestingT instead of testing.T to allow unit-testing.
type TestingT interface {
	Helper()
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}
