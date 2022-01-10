// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"time"
)

const (
	// defaultUIWaitTime indicates the default time to wait for UI elements to appear.
	defaultUIWaitTime = 5 * time.Second
	// longerUIWaitTime indicates the time to wait for some UI elements that need more time to appear.
	longerUIWaitTime = time.Minute
)

// Workload indicates the workload of the case.
type Workload uint8

// Browsering is for testing the browsering workload.
// Collaborating is for testing the collaborating workload.
const (
	Browsering Workload = iota
	Collaborating
)
