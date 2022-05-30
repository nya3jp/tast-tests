// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateengine provides ways to interact with update_engine daemon and utilities.
package updateengine

// Feature is the type of feature used internally during tast.
type Feature string

// List of features update_engine currently supports.
const (
	ConsumerAutoUpdate Feature = "feature-consumer-auto-update"
)
