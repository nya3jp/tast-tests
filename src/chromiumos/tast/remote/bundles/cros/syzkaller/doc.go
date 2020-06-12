// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package power contains Tast test wrapper around syzkaller.
//
// This wrapper runs syzkaller against the DUT for a duration of 10 minutes before
// stopping. The overall test duration is 12 minutes.
package syzkaller
