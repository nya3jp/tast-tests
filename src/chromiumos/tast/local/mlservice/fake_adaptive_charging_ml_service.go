// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mlservice provides functionality for modifying ml-service behavior
// for local tests.
package mlservice

import (
	"context"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/golang/protobuf/proto"

	pb "chromiumos/system_api/ml_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// The DBus library needs some local exported object to call
// RequestAdaptiveChargingDecision on.
type localUnused struct{}

// RequestAdaptiveChargingDecision always reports that the charger is expected
// to be unplugged in >8 hours.
func (l localUnused) RequestAdaptiveChargingDecision(serializedExampleProto []byte) (bool, []float64, *dbus.Error) {
	return true, []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0}, nil
}

// FakeAdaptiveChargingMLService Sets up a fake DBus service, which reports ML
// predictions as defined in RequestAdaptiveChargingDecision. Returns when the
// deadline for ctx is reached.
func FakeAdaptiveChargingMLService(ctx context.Context) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return errors.Wrap(err, "failed to connect to DBus system bus")
	}
	defer conn.Close()

	var status bool
	var result []float64
	example := &pb.RankerExample{}
	input, err := proto.Marshal(example)
	if err != nil {
		return errors.Wrap(err, "failed to Marshal protobuf")
	}
	obj := conn.Object("org.chromium.MachineLearning.AdaptiveCharging", "/org/chromium/MachineLearning/AdaptiveCharging")
	if err := obj.CallWithContext(ctx, "org.chromium.MachineLearning.AdaptiveCharging.RequestAdaptiveChargingDecision", 0, &input).Store(&status, &result); err != nil {
		return errors.Wrap(err, "failed to fetch Adaptive Charging prediction from ML service")
	}
	if status != true {
		return errors.New("ML Service failed to complete inference")
	}
	if len(result) != 9 {
		return errors.Wrapf(err, "ML service returned a float64 array of length %d instead of 9 as expected", len(result))
	}

	if err := upstart.StopJob(ctx, "ml-service", upstart.WithArg("TASK", "adaptive_charging")); err != nil {
		return errors.Wrap(err, "failed to stop ml-service TASK=adaptive_charging for starting fake service")
	}
	l := localUnused{}
	intro := &introspect.Node{
		Name: "/org/chromium/MachineLearning/AdaptiveCharging",
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{
				Name: "org.chromium.MachineLearning.AdaptiveCharging",
				Methods: []introspect.Method{
					{
						Name: "RequestAdaptiveChargingDecision",
						Args: []introspect.Arg{
							{
								Name:      "serialized_example_proto",
								Type:      "ay",
								Direction: "in",
							},
							{
								Name:      "status",
								Type:      "b",
								Direction: "out",
							},
							{
								Name:      "result",
								Type:      "ad",
								Direction: "out",
							},
						},
					},
				},
			},
		},
	}
	conn.Export(l, "/org/chromium/MachineLearning/AdaptiveCharging", "org.chromium.MachineLearning.AdaptiveCharging")
	conn.Export(introspect.NewIntrospectable(intro), "/org/chromium/MachineLearning/AdaptiveCharging", "org.freedesktop.DBus.Introspectable")

	reply, err := conn.RequestName("org.chromium.MachineLearning.AdaptiveCharging", dbus.NameFlagDoNotQueue)
	if err != nil {
		return errors.Wrap(err, "failed to request Fake ML Service name")
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return errors.New("ML service name already taken")
	}

	testing.ContextLog(ctx, "Listening on org.chromium.MachineLearning.AdaptiveCharging as fake Adaptive Charging ML service")

	select {
	case <-ctx.Done():
		return nil
	}
}
