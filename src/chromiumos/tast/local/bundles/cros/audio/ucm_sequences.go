// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

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
				Val: ucmSequencesParam{
					alsaucmCommander: staticCommander{
						{
							name:      "SectionVerb",
							extraArgs: nil,
						},
					},
				},
			},
			{
				Name:              "speaker",
				ExtraHardwareDeps: hwdep.D(hwdep.Speaker()),
				Val: ucmSequencesParam{
					alsaucmCommander: staticCommander{
						{
							name:      "EnableSequence",
							extraArgs: []string{"set", "_enadev", "Speaker"},
						},
						{
							name:      "DisableSequence",
							extraArgs: []string{"set", "_disdev", "Speaker"},
						},
					},
				},
			},
			{
				Name:              "internal_mic",
				ExtraHardwareDeps: hwdep.D(hwdep.Microphone()),
				Val: ucmSequencesParam{
					alsaucmCommander: staticCommander{
						{
							name:      "EnableSequence",
							extraArgs: []string{"set", "_enadev", "Internal Mic"},
						},
						{
							name:      "DisableSequence",
							extraArgs: []string{"set", "_disdev", "Internal Mic"},
						},
					},
				},
			},
			{
				Name: "section_modifier",
				Val: ucmSequencesParam{
					alsaucmCommander: modifierCommander{},
				},
			},
		},
	})
}

type ucmSequencesParam struct {
	alsaucmCommander
}

// alsaucmCommander provides commands() that returns a list of alsaucmCommand,
// which can be used to generate alsaucm command to run.
type alsaucmCommander interface {
	commands(ctx context.Context, ucmName string) ([]alsaucmCommand, error)
}

type alsaucmCommand struct {
	name      string   // human readable name describing the command
	extraArgs []string // extra args after `alsaucm -c$cardname set _verb HiFi`
}

// staticCommander is an alsaucmCommander that returns a predetermined list of alsaucmCommand.
type staticCommander []alsaucmCommand

var _ alsaucmCommander = staticCommander{}

func (p staticCommander) commands(ctx context.Context, ucmName string) ([]alsaucmCommand, error) {
	return p, nil
}

// modifierCommander is an alsaucmCommander that returns a list of alsaucmCommand
// based on available "SectionModifier"s in HiFi.conf.
type modifierCommander struct{}

var _ alsaucmCommander = modifierCommander{}

func (p modifierCommander) commands(ctx context.Context, ucmName string) ([]alsaucmCommand, error) {
	cmd := testexec.CommandContext(ctx,
		"alsaucm",
		"-c"+ucmName,
		"list",
		"_modifiers/HiFi",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list modifiers")
	}

	// Find the names listed after the ": "
	// example:
	// 0: Hotword Model ar_sa
	// 1: Hotword Model cmn
	var commands []alsaucmCommand
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			modifier := parts[1]
			commands = append(commands, alsaucmCommand{modifier, []string{"set", "_enamod", modifier}})
		}
	}
	return commands, nil
}

func UCMSequences(ctx context.Context, s *testing.State) {
	// Since we are messing with mixer controls, restart CRAS after running the test.
	clearUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(clearUpCtx, 5*time.Second)
	defer cancel()
	defer audio.RestartCras(clearUpCtx)

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
		ucmSequencesTestCard(ctx, s, param.alsaucmCommander, ucmName)
	}
}

func ucmSequencesTestCard(ctx context.Context, s *testing.State, c alsaucmCommander, ucmName string) {
	const ucmBasePath = "/usr/share/alsa/ucm"

	s.Logf("Testing UCM: %s", ucmName)

	// Check we have a complete HiFi.conf.
	// If the file is absent or empty (placeholder during bringup), skip the test.
	hifiConf := filepath.Join(ucmBasePath, ucmName, "HiFi.conf")
	if stat, err := os.Stat(hifiConf); err != nil {
		if os.IsNotExist(err) {
			s.Log("Skipping due to missing HiFi.conf")
			return
		}
		if stat.Size() == 0 {
			s.Log("Skipping due to empty HiFi.conf")
			return
		}
		s.Fatal("Failed to stat HiFi.conf: ", err)
	}

	ucmCmds, err := c.commands(ctx, ucmName)
	if err != nil {
		s.Error("Cannot get UCM commands: ", err)
		return
	}
	for _, ucmCmd := range ucmCmds {
		cmd := testexec.CommandContext(ctx,
			"alsaucm",
			"-c"+ucmName,
			"set",
			"_verb",
			"HiFi",
		)
		cmd.Args = append(cmd.Args, ucmCmd.extraArgs...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			s.Errorf("%s %s failed:%s%s",
				cmd.Path, strings.Join(cmd.Args, " "), "\n", string(out))
		}
	}
}
