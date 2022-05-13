// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dbusutil provides additional functionality on top of the godbus/dbus package.
package dbusutil

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// The FakeUser/GuestUser are used to simulate a regular/guest user login.
const (
	fakeEndSignal = "FakeEndSignal"
)

// The CalledMethod struct represents a method call and arguments that was observed by DbusEventMonitor.
type CalledMethod struct {
	MethodName string
	Arguments  []interface{}
}

// DbusEventMonitor monitors the system message bus for the D-Bus calls we want to observe as specified in |specs|.
// It returns a stop function and error. The stop function stops the D-Bus monitor and return the called methods and/or error.
func DbusEventMonitor(ctx context.Context, specs []MatchSpec) (func() ([]CalledMethod, error), error) {
	ch := make(chan error, 1)
	var calledMethods []CalledMethod
	stop := func() ([]CalledMethod, error) {
		// Send a fake dbus signal to stop the Eavesdrop.
		connect, err := SystemBus()
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to system bus")
		}
		if err := connect.Emit("/", fmt.Sprintf("com.fake.%s", fakeEndSignal)); err != nil {
			return calledMethods, errors.Wrap(err, "failed sending fake signal to stop Eavesdrop")
		}
		if err := <-ch; err != nil {
			return calledMethods, err
		}
		return calledMethods, nil
	}

	conn, err := SystemBusPrivate()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system bus")
	}
	err = conn.Auth(nil)
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "failed to authenticate the system bus")
	}
	err = conn.Hello()
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "failed to send the Hello call to the system bus")
	}

	specs = append(specs, MatchSpec{
		Type:      "signal",
		Interface: "com.fake",
		Member:    fakeEndSignal,
	})

	var rules []string
	var allowlistDbusCmd []string
	for _, spec := range specs {
		rules = append(rules, spec.String())
		allowlistDbusCmd = append(allowlistDbusCmd, spec.Member)
	}

	call := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, uint(0))
	if call.Err != nil {
		return nil, errors.Wrap(call.Err, "failed to become monitor")
	}

	c := make(chan *dbus.Message, 10)
	conn.Eavesdrop(c)

	go func() {
		defer func() {
			conn.Eavesdrop(nil)
			conn.Close()
		}()

		for {
			select {
			case <-ctx.Done():
				ch <- errors.New("failed waiting for signal")
			case msg := <-c:
				dbusCmd, err := dbusCallMember(msg, allowlistDbusCmd)
				if err != nil {
					testing.ContextLog(ctx, "Something failed: ", err)
					continue
				}
				if dbusCmd == fakeEndSignal {
					ch <- nil
					return
				}
				calledMethods = append(calledMethods, CalledMethod{dbusCmd, msg.Body})
			}
		}
	}()

	return stop, nil
}

// dbusCallMember returns the member name of the D-Bus call.
func dbusCallMember(dbusMessage *dbus.Message, allowlistDbusCmd []string) (string, error) {
	v, ok := dbusMessage.Headers[dbus.FieldMember]
	if !ok {
		return "", errors.Errorf("failed dbus message doesn't have field member: %s", dbusMessage)
	}
	msg := fmt.Sprintf(v.String()[1 : len(v.String())-1])
	for _, cmd := range allowlistDbusCmd {
		if msg == cmd {
			return cmd, nil
		}
	}
	return "", errors.Errorf("failed found unexpected call: got %s, want %v", msg, allowlistDbusCmd)
}
