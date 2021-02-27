// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfutil

// CQTargetBoards defines the list of boards to be verified for chromeos-chorme
// uprev and chromium postsubmit bots for perf tests. Since perf tests need
// actual physical displays to collect data points, VMs (e.g. amd64-generic or
// bettry) are not included.
var CQTargetBoards = []string{
	// Boards used for the CQ of chromeos-chrome uprevs.
	"atlas",
	"coral",
	"grunt",
	"hatch",
	"octopus",
	"scarlet",
	// In chromium postsubmit bots.
	"eve",
	"kevin",
}
