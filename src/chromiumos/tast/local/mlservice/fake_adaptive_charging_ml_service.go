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
	conn *dbus.Conn
}

const (
	iface      = "org.chromium.MachineLearning.AdaptiveCharging"
	introIface = "org.freedesktop.DBus.Introspectable"
	path       = "/org/chromium/MachineLearning/AdaptiveCharging"
	member     = "RequestAdaptiveChargingDecision"
)

// RequestAdaptiveChargingDecision always reports that the charger is expected
// to be unplugged in >8 hours.
func (f *FakeAdaptiveChargingMLService) RequestAdaptiveChargingDecision(serializedExampleProto []byte) (bool, []float64, *dbus.Error) {
	return true, []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0}, nil
}

// StartFakeAdaptiveChargingMLService starts a fake version of the ml-service
// TASK=adaptive_charging. This service will stop when StopService is called on
// the returned *FakeAdaptiveChargingMLService. If the returned error is not
// nil, the *FakeAdaptiveChargingMLService will be nil.
func StartFakeAdaptiveChargingMLService(ctx context.Context) (*FakeAdaptiveChargingMLService, error) {
	f := new(FakeAdaptiveChargingMLService)
	var err error
	f.conn, err = dbus.ConnectSystemBus()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to DBus system bus")
	}

	var status bool
	var result []float64
	example := &pb.RankerExample{}
	input, err := proto.Marshal(example)
	if err != nil {
		return nil, errors.Wrap(err, "failed to Marshal protobuf")
	}
	obj := f.conn.Object(iface, path)
	if err := obj.CallWithContext(ctx, iface+"."+member, 0, &input).Store(&status, &result); err != nil {
		return nil, errors.Wrap(err, "failed to fetch Adaptive Charging prediction from ML service")
	}
	if !status {
		return nil, errors.New("ML service failed to complete inference")
	}
	if len(result) != 9 {
		return nil, errors.Wrapf(err, "ML service returned a float64 array of length %d instead of 9 as expected", len(result))
	}

	// There's no need to restore this ml-service after we're done, since it's
	// created on-demand if it doesn't exist.
	if err := upstart.StopJob(ctx, "ml-service", upstart.WithArg("TASK", "adaptive_charging")); err != nil {
		return nil, errors.Wrap(err, "failed to stop ml-service TASK=adaptive_charging for starting fake service")
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
	f.conn.Export(f, path, iface)
	f.conn.Export(introspect.NewIntrospectable(intro), path, introIface)

	reply, err := f.conn.RequestName(iface, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request Fake ML Service name")
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, errors.New("ML service name already taken")
	}

	testing.ContextLog(ctx, "Listening on "+iface+" as fake Adaptive Charging ML service")
	return f, nil
}

// StopService removes the exported objects and methods, then closes the DBus
// connection, stopping the fake service.
func (f *FakeAdaptiveChargingMLService) StopService() {
	f.conn.Export(nil, path, introIface)
	f.conn.Export(nil, path, iface)
	f.conn.Close()
}
