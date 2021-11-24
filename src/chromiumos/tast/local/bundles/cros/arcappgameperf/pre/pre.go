// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre provides preconditions to be used by game performance related tests.
package pre

import "chromiumos/tast/local/arc"

var arcappgameperf = &arc.GaiaVars{
	UserVar: "arcappgameperf.username",
	PassVar: "arcappgameperf.password",
}

// ArcAppGamePerfBooted is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the game perf credentials, and opt-in to the Play Store.
var ArcAppGamePerfBooted = arc.NewPrecondition("arcappgameperf_booted", arcappgameperf, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)
