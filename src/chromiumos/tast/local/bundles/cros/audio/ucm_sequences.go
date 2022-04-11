// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type ucmSequencesParam struct {
	sectionDevice string // UCM SectionDevice to test. If empty test SectinoVerb only.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     UCMSequences,
		Desc:     "Exercise UCM config enable/disable sequences",
		Contacts: []string{"aaronyu@google.com", "chromeos-audio-bugs@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  1 * time.Minute,
		Params: []testing.Param{
			{
				Name: "section_verb",
				Val:  ucmSequencesParam{},
			},
			{
				Name:              "speaker",
				ExtraHardwareDeps: hwdep.D(hwdep.Speaker()),
				Val: ucmSequencesParam{
					sectionDevice: "Speaker",
				},
			},
			{
				Name:              "internal_mic",
				ExtraHardwareDeps: hwdep.D(hwdep.Microphone()),
				Val: ucmSequencesParam{
					sectionDevice: "Internal Mic",
				},
			},
		},
	})
}

func UCMSequences(ctx context.Context, s *testing.State) {
	param := s.Param().(ucmSequencesParam)

	cards, err := audio.GetSoundCards()
	if err != nil {
		s.Fatal("Failed to get sound cards: ", err)
	}

	ucmSuffix, err := crosconfig.Get(ctx, "/audio/main", "ucm-suffix")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Cannot get ucm suffix: ", err)
	}

	for _, card := range cards {
		ucmName := card.ShortName
		if ucmSuffix != "" {
			ucmName += "." + ucmSuffix
		}
		ucmSequencesTestCard(ctx, s, param.sectionDevice, ucmName)
	}
}

func ucmSequencesTestCard(ctx context.Context, s *testing.State, sectionDevice, ucmName string) {
	const ucmBasePath = "/usr/share/alsa/ucm"

	s.Logf("Testing UCM: %s; SectionDevice: %q", ucmName, sectionDevice)
	hifiConf := filepath.Join(ucmBasePath, ucmName, "HiFi.conf")
	b, err := os.ReadFile(hifiConf)
	if err != nil && os.IsNotExist(err) {
		s.Log("Skipping due to missing HiFi.conf")
		return
	}
	if err != nil {
		s.Error("Failed to read HiFi.conf: ", err)
	}

	if !bytes.Contains(b, []byte("SectionVerb")) {
		s.Log("Skipping due to missing SectionVerb in HiFi.conf")
		return
	}

	runSequence := func(name string, extraArgs []string) {
		cmd := testexec.CommandContext(ctx,
			"alsaucm",
			"-c"+ucmName,
			"set",
			"_verb",
			"HiFi",
		)
		cmd.Args = append(cmd.Args, extraArgs...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			s.Errorf("%s %s failed:%s%s",
				cmd.Path, strings.Join(cmd.Args, " "), "\n", string(out))
		}
	}
	if sectionDevice == "" {
		runSequence("SectionVerb", nil)
	} else {
		runSequence("EnableSequence", []string{"set", "_enadev", sectionDevice})
		runSequence("DisableSequence", []string{"set", "_disdev", sectionDevice})
	}
}
