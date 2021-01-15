// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver provides components to interact with a locally-running Chrome
// process in various ways, including DevTools protocol and /proc monitoring.
//
// This package works with a properly set up Chrome processes. This package does
// NOT support setting up / starting Chrome processes.
package driver
