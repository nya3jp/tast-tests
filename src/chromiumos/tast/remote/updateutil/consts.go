// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateutil

import "time"

// UpdateTimeout is the time needed to download image and install it to DUT.
const UpdateTimeout = 25 * time.Minute
