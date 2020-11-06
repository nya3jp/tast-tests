// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shillconst

// IsConnectedState checks if the service state is in
// ServiceConnectedStates.
func IsConnectedState(state string) bool {
	for _, s := range ServiceConnectedStates {
		if state == s.(string) {
			return true
		}
	}
	return false
}
