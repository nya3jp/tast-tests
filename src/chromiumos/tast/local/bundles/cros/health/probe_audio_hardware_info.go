// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/health/types"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type audioHardwareInfo struct {
	AudioCards []audioCard `json:"audio_cards"`
}

type audioCard struct {
	AlsaID        string           `json:"alsa_id"`
	BusDevice     *types.BusDevice `json:"bus_device"`
	HDAudioCodecs []hdAudioCodec   `json:"hd_audio_codecs"`
}

type hdAudioCodec struct {
	Name    string `json:"name"`
	Address uint8  `json:"address"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeAudioHardwareInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for audio hardware info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func validate(info audioHardwareInfo) error {
	for _, audioCard := range info.AudioCards {
		if audioCard.BusDevice != nil && audioCard.BusDevice.DeviceClass != "audio card" {
			return errors.Errorf("DeviceClass is not audio card: %s", audioCard.BusDevice.DeviceClass)
		}
	}

	return nil
}

func ProbeAudioHardwareInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryAudioHardware}
	var info audioHardwareInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get audio hardware telemetry info: ", err)
	}

	if err := validate(info); err != nil {
		s.Fatalf("Failed to validate, err [%v]", err)
	}
}
