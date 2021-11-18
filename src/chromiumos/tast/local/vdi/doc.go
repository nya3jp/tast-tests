// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vdi contains fixtures for starting VDI applications (e.g. VMware,
// Citrix) in various available configurations: user session, Kiosk sessions,
// and MGS - manage guest session. It also provides an interface definition for
// interacting with VDI apps and its implementation apps/vmware.go and
// apps/citrix.go. Along with the implementation it stores the UI fragments in
// fixtures/data that are used to identify what to interact from tests.
package vdi
