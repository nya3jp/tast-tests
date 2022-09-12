// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"strconv"

	"chromiumos/tast/testing"
)

const (
	numBotsVarName = "numBots"
	presentVarName = "present"
	webRTCFileName = "webrtc-internals.json"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestReportWebRTCInternals,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the reportWebRTCInternals function",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Vars:         []string{numBotsVarName, presentVarName},
		Data:         []string{webRTCFileName},
	})
}

func TestReportWebRTCInternals(ctx context.Context, s *testing.State) {
	numBots, err := strconv.Atoi(s.RequiredVar(numBotsVarName))
	if err != nil {
		s.Fatalf("Failed to parse %q variable: %s", numBotsVarName, err)
	}

	_, present := s.Var(presentVarName)

	dump, err := os.ReadFile(s.DataPath(webRTCFileName))
	if err != nil {
		s.Fatalf("Failed to read WebRTC internals dump from %q: %s", webRTCFileName, err)
	}

	pv, err := reportWebRTCInternals(dump, numBots, present)
	if err != nil {
		s.Fatal("reportWebRTCInternals returned a non-nil error: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
