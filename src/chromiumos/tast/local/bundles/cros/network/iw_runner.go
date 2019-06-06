// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

/*
This file serves as a wrapper to allow tast tests to query 'iw' for network device characteristics.
iw_runner.go is the analog of {@link iw_runner.py} in the autotest suite.
*/
