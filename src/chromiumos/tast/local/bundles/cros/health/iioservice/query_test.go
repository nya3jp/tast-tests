// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iioservice

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var iioserviceQueryRes = `
2022-09-29T03:01:04.415686Z INFO iioservice_query: [query_impl.cc(120)] (30747) GetAttributesCallback(): Device id: 0
2022-09-29T03:01:04.415721Z INFO iioservice_query: [query_impl.cc(123)] (30747) GetAttributesCallback(): Type: ACCEL
2022-09-29T03:01:04.415725Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): name: cros-ec-accel
2022-09-29T03:01:04.415728Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): location: lid
2022-09-29T03:01:04.415755Z INFO iioservice_query: [query_impl.cc(120)] (30747) GetAttributesCallback(): Device id: 1
2022-09-29T03:01:04.415761Z INFO iioservice_query: [query_impl.cc(123)] (30747) GetAttributesCallback(): Type: ANGL
2022-09-29T03:01:04.415765Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): name: cros-ec-lid-angle
2022-09-29T03:01:04.415769Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): location: 
2022-09-29T03:01:04.415800Z INFO iioservice_query: [query_impl.cc(120)] (30747) GetAttributesCallback(): Device id: 2
2022-09-29T03:01:04.415805Z INFO iioservice_query: [query_impl.cc(123)] (30747) GetAttributesCallback(): Type: ACCEL
2022-09-29T03:01:04.415809Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): name: cros-ec-accel
2022-09-29T03:01:04.415812Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): location: base
2022-09-29T03:01:04.415872Z INFO iioservice_query: [query_impl.cc(120)] (30747) GetAttributesCallback(): Device id: 3
2022-09-29T03:01:04.415883Z INFO iioservice_query: [query_impl.cc(123)] (30747) GetAttributesCallback(): Type: ANGLVEL
2022-09-29T03:01:04.415889Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): name: cros-ec-gyro
2022-09-29T03:01:04.415895Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): location: base
2022-09-29T03:01:04.415931Z INFO iioservice_query: [query_impl.cc(120)] (30747) GetAttributesCallback(): Device id: 4
2022-09-29T03:01:04.415936Z INFO iioservice_query: [query_impl.cc(123)] (30747) GetAttributesCallback(): Type: LIGHT
2022-09-29T03:01:04.415942Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): name: acpi-als
2022-09-29T03:01:04.415947Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): location: 
2022-09-29T03:01:04.416004Z INFO iioservice_query: [query_impl.cc(120)] (30747) GetAttributesCallback(): Device id: 10000
2022-09-29T03:01:04.416010Z INFO iioservice_query: [query_impl.cc(123)] (30747) GetAttributesCallback(): Type: GRAVITY
2022-09-29T03:01:04.416016Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): name: iioservice-gravity
2022-09-29T03:01:04.416021Z INFO iioservice_query: [query_impl.cc(126)] (30747) GetAttributesCallback(): location: base
2022-09-29T03:01:04.416043Z INFO iioservice_query: [daemon.cc(47)] (30747) OnMojoDisconnect(): Quitting this process.
`

func TestExpectedSensorAttributes(t *testing.T) {
	iioserviceQueryCmd = func(ctx context.Context) ([]byte, []byte, error) {
		return nil, []byte(iioserviceQueryRes), nil
	}
	got, err := ExpectedSensorAttributes(context.Background())
	if err != nil {
		t.Fatal("Failed to run ExpectedSensorAttributes: ", err)
	}

	sensorNames := []string{"cros-ec-accel", "cros-ec-lid-angle", "cros-ec-accel", "cros-ec-gyro", "acpi-als", "iioservice-gravity"}
	expected := []SensorAttributes{
		SensorAttributes{
			Name:     &sensorNames[0],
			DeviceID: 0,
			Type:     "Accel",
			Location: "Lid",
		},
		SensorAttributes{
			Name:     &sensorNames[1],
			DeviceID: 1,
			Type:     "Angle",
			Location: "Unknown",
		},
		SensorAttributes{
			Name:     &sensorNames[2],
			DeviceID: 2,
			Type:     "Accel",
			Location: "Base",
		},
		SensorAttributes{
			Name:     &sensorNames[3],
			DeviceID: 3,
			Type:     "Gyro",
			Location: "Base",
		},
		SensorAttributes{
			Name:     &sensorNames[4],
			DeviceID: 4,
			Type:     "Light",
			Location: "Unknown",
		},
		SensorAttributes{
			Name:     &sensorNames[5],
			DeviceID: 10000,
			Type:     "Gravity",
			Location: "Base",
		},
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatal("Iioservice query test failed (-expected + got): ", diff)
	}
}
