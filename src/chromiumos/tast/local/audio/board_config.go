// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
)

// BoardConfig represents a board config file in /etc/cras/${board}/board.ini
type BoardConfig struct {
	ucm boardConfigUCM
}

// BoardConfig represents the [ucm] section in BoardConfig
type boardConfigUCM struct {
	// Cards whose ucm-suffix should be ignored
	// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/third_party/adhd/cras/src/server/cras_system_state.c?q=symbol:init_ignore_suffix_cards&ss=chromiumos%2Fchromiumos%2Fcodesearch:src%2Fthird_party%2Fadhd%2Fcras%2F
	IgnoreSuffixList []string
}

func parseBoardConfig(b []byte) (*BoardConfig, error) {
	cfg, err := ini.Load(b)
	if err != nil {
		return nil, err
	}

	var ucmIgnoreSuffixList []string
	if rawIgnoreSuffix := cfg.Section("ucm").Key("ignore_suffix").String(); rawIgnoreSuffix != "" {
		ucmIgnoreSuffixList = strings.Split(rawIgnoreSuffix, ",")
	}

	return &BoardConfig{
		ucm: boardConfigUCM{
			IgnoreSuffixList: ucmIgnoreSuffixList,
		},
	}, nil
}

// LoadBoardConfig loads the board config for the system.
func LoadBoardConfig(ctx context.Context) (*BoardConfig, error) {
	crasConfigDir, err := crosconfig.Get(ctx, "/audio/main", "cras-config-dir")
	if err != nil {
		return nil, errors.Wrap(err, "cannot get audio board name")
	}

	configPath := filepath.Join("/etc/cras", crasConfigDir, "board.ini")
	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	return parseBoardConfig(b)
}

// ShouldIgnoreUCMSuffix returns whether the ucm-suffix of card should be ignored.
// This function behaves the same as the cras_system_check_ignore_ucm_suffix() function.
func (c *BoardConfig) ShouldIgnoreUCMSuffix(card string) bool {
	if card == "Loopback" {
		return true
	}
	for _, ignore := range c.ucm.IgnoreSuffixList {
		if card == ignore {
			return true
		}
	}
	return false
}
