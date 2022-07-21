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

// FakeAdaptiveChargingMLService manages a fake DBus service which replaces the
// ml-service TAST=adaptive_charging
type FakeAdaptiveChargingMLService struct {
	hasError chan error
	done     chan bool
}

// StartFakeAdaptiveChargingMLService starts a fake version of the ml-service
// TASK=adaptive_charging. This service will stop when StopService is called on
// the returned *FakeAdaptiveChargingMLService or when ctx is Done. If the
// returned error is not nil, there's no need to call StopService.
func StartFakeAdaptiveChargingMLService(ctx context.Context) (*FakeAdaptiveChargingMLService, error) {
	f := new(FakeAdaptiveChargingMLService)
	f.done = make(chan bool)
	f.hasError = make(chan error, 1)

	go f.serviceRoutine(ctx)

	// Return the FakeAdaptiveChargingMLService object and an error (if there is
	// one) from starting the service.
	return f, <-f.hasError
}

// RequestAdaptiveChargingDecision always reports that the charger is expected
// to be unplugged in >8 hours.
func (f FakeAdaptiveChargingMLService) RequestAdaptiveChargingDecision(serializedExampleProto []byte) (bool, []float64, *dbus.Error) {
	return true, []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0}, nil
}

// serviceRoutine sets up a fake DBus service, which reports ML predictions as
// defined in RequestAdaptiveChargingDecision. Returns when the deadline for ctx
// is reached, or if f.done is set to true. Reports any errors via f.hasError.
func (f FakeAdaptiveChargingMLService) serviceRoutine(ctx context.Context) {
	const (
		iface  = "org.chromium.MachineLearning.AdaptiveCharging"
		path   = "/org/chromium/MachineLearning/AdaptiveCharging"
		member = "RequestAdaptiveChargingDecision"
	)

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		f.hasError <- errors.Wrap(err, "failed to connect to DBus system bus")
		return
	}
	defer conn.Close()

	var status bool
	var result []float64
	example := &pb.RankerExample{}
	input, err := proto.Marshal(example)
	if err != nil {
		f.hasError <- errors.Wrap(err, "failed to Marshal protobuf")
		return
	}
	obj := conn.Object(iface, path)
	if err := obj.CallWithContext(ctx, iface+"."+member, 0, &input).Store(&status, &result); err != nil {
		f.hasError <- errors.Wrap(err, "failed to fetch Adaptive Charging prediction from ML service")
		return
	}
	if !status {
		f.hasError <- errors.New("ML service failed to complete inference")
		return
	}
	if len(result) != 9 {
		f.hasError <- errors.Wrapf(err, "ML service returned a float64 array of length %d instead of 9 as expected", len(result))
		return
	}

	// There's no need to restore this ml-service after we're done, since it's
	// created on-demand if it doesn't exist.
	if err := upstart.StopJob(ctx, "ml-service", upstart.WithArg("TASK", "adaptive_charging")); err != nil {
		f.hasError <- errors.Wrap(err, "failed to stop ml-service TASK=adaptive_charging for starting fake service")
	}

	intro := &introspect.Node{
		Name: path,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{
				Name: iface,
				Methods: []introspect.Method{
					{
						Name: member,
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
	conn.Export(f, path, iface)
	conn.Export(introspect.NewIntrospectable(intro), path, "org.freedesktop.DBus.Introspectable")

	reply, err := conn.RequestName(iface, dbus.NameFlagDoNotQueue)
	if err != nil {
		f.hasError <- errors.Wrap(err, "failed to request Fake ML Service name")
		return
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		f.hasError <- errors.New("ML service name already taken")
		return
	}

	f.hasError <- nil
	testing.ContextLog(ctx, "Listening on "+iface+" as fake Adaptive Charging ML service")

	select {
	case <-ctx.Done():
		return
	case <-f.done:
		return
	}
}

// StopService sets the channel done to false, which causes the
// FakeAdaptiveChargingMLService to stop and return.
func (f FakeAdaptiveChargingMLService) StopService() {
	f.done <- true
}
