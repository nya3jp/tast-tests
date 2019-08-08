// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains local Tast tests that exercise Chrome OS crash handling.
//
// These tests typically perform operations that are risky or impossible to do in a local test
// such as panicking or otherwise forcibly rebooting the DUT.
package crash
