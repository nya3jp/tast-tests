// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dep

import (
	"chromiumos/tast/testing/hwdep"
)

// List of models that pass the telemetryextension.HasOEMName test.
// In general it's expected that Telemetry Extension Platform tests will
// consistently pass on these models.
var stableModelList = []string{
	// HP models:
	"vell",
	"dojo",
	"redrix",
}

// StableModels returns hardwareDeps condition with list of stable models.
func StableModels() hwdep.Deps {
	return hwdep.D(hwdep.Model(stableModelList...))
}
