// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package params

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:         fixture.Omaha,
		Desc:         "Fixture that loads data for omaha tests",
		Contacts:     []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:         &omahaFixture{},
		SetUpTimeout: 20 * time.Second,
		Data:         []string{"configuration.json", "pins.json"},
	})
}

type omahaFixture struct{}

// FixtData is the data made available by the fixture for its tests.
// Contains DUT parameters and the current configuration.
type FixtData struct {
	Device           *Device
	Config           *Configuration
	MinorVersionPins []MinorVersionPin
}

func (o *omahaFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Load DUT params.
	dut, err := loadParamsFromDUT(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to load device parameters: ", err)
	}

	if err := dut.DumpToFile(filepath.Join(s.OutDir(), "device-param.json")); err != nil {
		s.Error("Failed to dump 'device-param.json': ", err)
	}

	// Load Configuration.
	b, err := ioutil.ReadFile(s.DataPath("configuration.json"))

	var config Configuration
	if err := json.Unmarshal(b, &config); err != nil {
		s.Fatal("Failed to parse the configuration: ", err)
	}

	if err := config.DumpToFile(filepath.Join(s.OutDir(), "configuration.json")); err != nil {
		s.Error("Failed to dump 'configuration.json': ", err)
	}

	b, err = ioutil.ReadFile(s.DataPath("pins.json"))

	// Load Minor Version Pins.
	var pins []MinorVersionPin
	if err := json.Unmarshal(b, &pins); err != nil {
		s.Fatal("Failed to parse the pins: ", err)
	}

	file, err := json.MarshalIndent(pins, "", "  ")
	if err != nil {
		s.Fatal("Failed to marshall pins: ", err)
	}
	ioutil.WriteFile(filepath.Join(s.OutDir(), "pins.json"), file, 0644)

	// Send everything to the tests.
	return &FixtData{
		Device:           dut,
		Config:           &config,
		MinorVersionPins: pins,
	}
}

func (o *omahaFixture) TearDown(ctx context.Context, s *testing.FixtState) {}
func (o *omahaFixture) Reset(ctx context.Context) error {
	return nil
}
func (o *omahaFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (o *omahaFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
