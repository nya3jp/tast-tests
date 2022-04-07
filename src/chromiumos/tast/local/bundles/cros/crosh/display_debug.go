// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package displaydebug verifies display_debug commands.
package displaydebug

import (
	"context"
	"fmt"
	"reflect"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
)

const (
	drmTraceSetCategories = "DRMTraceSetCategories"
	drmTraceSetSize       = "DRMTraceSetSize"
	drmTraceAnnotateLog   = "DRMTraceAnnotateLog"
	drmTraceSnapshot      = "DRMTraceSnapshot"
)

// argumentVerifierFunc takes a D-Bus method argument as input, and returns an error if the value is
// not as expected, nil otherwise.
type argumentVerifierFunc func(argument interface{}) error

// expectedMethodCall represents the name of a D-Bus method and functions to verify each argument it was called with.
type expectedMethodCall struct {
	methodName    string
	verifierFuncs []argumentVerifierFunc
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DisplayDebug, LacrosStatus: testing.LacrosVariantUnneeded, Desc: "Tests the display_debug commands",
		Contacts: []string{
			"ddavenport@google.com",
			"chromeos-gfx-display@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "drm_trace"},
	})
}

func DisplayDebug(ctx context.Context, s *testing.State) {
	s.Log("Verify debug_display trace_start")
	if err := testTraceStart(ctx, s); err != nil {
		s.Error("Failed to verify `display_debug trace_start`: ", err)
	}
	s.Log("Verify debug_display trace_stop")
	if err := testTraceStop(ctx, s); err != nil {
		s.Error("Failed to verify `display_debug trace_stop`: ", err)
	}
}

func testTraceStart(ctx context.Context, s *testing.State) error {
	calledMethods, err := runCroshCommandTest(ctx, []string{"display_debug", "trace_start"})
	if err != nil {
		return err
	}

	if len(calledMethods) != 3 {
		return errors.Errorf("unexpected number of method calls. got %d, want %d", len(calledMethods), 3)
	}
	// First two method calls should be to set the size and categories. Their relative order does not matter.
	if err := verifyAnyMethodCall(calledMethods[:2],
		expectedMethodCall{
			drmTraceSetCategories, []argumentVerifierFunc{makeCheckNeqArgument(0)},
		}); err != nil {
		return err
	}
	if err := verifyAnyMethodCall(calledMethods[:2],
		expectedMethodCall{
			drmTraceSetSize, []argumentVerifierFunc{makeCheckEqArgument(debugd.DRMTraceSizeDebug)},
		}); err != nil {
		return err
	}
	// Last method call should be to annotate the log.
	if err := verifyMethodCall(calledMethods[2],
		expectedMethodCall{
			drmTraceAnnotateLog, []argumentVerifierFunc{nil},
		}); err != nil {
		return err
	}
	return nil
}

func testTraceStop(ctx context.Context, s *testing.State) error {
	calledMethods, err := runCroshCommandTest(ctx, []string{"display_debug", "trace_stop"})
	if err != nil {
		return err
	}

	if len(calledMethods) != 4 {
		return errors.Errorf("unexpected number of method calls. got %d, want %d", len(calledMethods), 3)
	}
	// First method call should be to annotate the log.
	if err := verifyMethodCall(calledMethods[0],
		expectedMethodCall{
			drmTraceAnnotateLog, []argumentVerifierFunc{nil},
		}); err != nil {
		return err
	}
	// Next is a log snapshot.
	if err := verifyMethodCall(calledMethods[1],
		expectedMethodCall{
			drmTraceSnapshot, []argumentVerifierFunc{makeCheckEqArgument(uint32(debugd.DRMTraceSnapshotTypeTrace))},
		}); err != nil {
		return err
	}
	// The last two method calls should reset the size and categories. Their relative order does not matter.
	if err := verifyAnyMethodCall(calledMethods[2:],
		expectedMethodCall{
			drmTraceSetCategories, []argumentVerifierFunc{makeCheckEqArgument(0)},
		}); err != nil {
		return err
	}
	if err := verifyAnyMethodCall(calledMethods[2:],
		expectedMethodCall{
			drmTraceSetSize, []argumentVerifierFunc{makeCheckEqArgument(uint32(debugd.DRMTraceSizeDefault))},
		}); err != nil {
		return err
	}
	return nil
}

// runCroshCommandTest sets up a dbusutil.DbusEventMonitor to eavesdrop on method calls, and runs |croshCommand|. It
// returns []dbusutil.CalledMethod of all the methods called while the crosh command ran.
func runCroshCommandTest(ctx context.Context, croshCommand []string) ([]dbusutil.CalledMethod, error) {
	var specs []dbusutil.MatchSpec
	for _, method := range []string{drmTraceAnnotateLog, drmTraceSetCategories, drmTraceSetSize, drmTraceSnapshot} {
		specs = append(specs, dbusutil.MatchSpec{
			Type:      "method_call",
			Interface: "org.chromium.debugd",
			Member:    method,
		})
	}

	stop, err := dbusutil.DbusEventMonitor(ctx, specs)
	if err != nil {
		return nil, err
	}

	command := append([]string{"crosh", "--dev=false", "--"}, croshCommand...)
	if err := testexec.CommandContext(ctx, command[0], command[1:]...).Run(); err != nil {
		stop()
		return nil, errors.Wrap(err, "failed to run `display_debug trace_start` command")
	}

	return stop()
}

// makeCheckEqArgument returns an argumentVerifierFunc which will verify that the argument is equal to |value|.
func makeCheckEqArgument(value uint32) argumentVerifierFunc {
	return func(argument interface{}) error {
		argument, ok := argument.(uint32)
		if !ok {
			return errors.Errorf("unexepected argument type. got %s, want uint32", fmt.Sprint(reflect.TypeOf(argument)))
		}
		if argument != value {
			return errors.Errorf("unexpected argument: got %v, want %v ", argument, value)
		}
		return nil
	}
}

// makeCheckNeqArgument returns an argumentVerifierFunc which will verify that the argument is not equal to |value|.
func makeCheckNeqArgument(value uint32) argumentVerifierFunc {
	return func(argument interface{}) error {
		argument, ok := argument.(uint32)
		if !ok {
			return errors.Errorf("unexepected argument type. got %s, want uint32", fmt.Sprint(reflect.TypeOf(argument)))
		}
		if argument == value {
			return errors.Errorf("unexpected argument: equal to %d", value)
		}
		return nil
	}
}

// verifyAnyMethodCall checks if any of |calledMethods| matches |expectedMethod|.
func verifyAnyMethodCall(calledMethods []dbusutil.CalledMethod, expectedMethod expectedMethodCall) error {
	var err error
	for _, method := range calledMethods {
		if err = verifyMethodCall(method, expectedMethod); err == nil {
			return nil
		}
	}
	if err == nil {
		return errors.Errorf("missing method call: %s", expectedMethod.methodName)
	}
	return err
}

// verifyMethodCall checks if |method| matches |expectedMethod|.
func verifyMethodCall(method dbusutil.CalledMethod, expectedMethod expectedMethodCall) error {
	if method.MethodName != expectedMethod.methodName {
		return errors.Errorf("missing method call: %s", expectedMethod.methodName)
	}

	if len(method.Arguments) != len(expectedMethod.verifierFuncs) {
		return errors.Errorf("number of argumentVerifiers(%d)) not equal to number of arguments(%d)", len(expectedMethod.verifierFuncs), len(method.Arguments))
	}
	for i, argument := range method.Arguments {
		if expectedMethod.verifierFuncs[i] != nil {
			if err := expectedMethod.verifierFuncs[i](argument); err != nil {
				return errors.Wrapf(err, "failed to validate arguments for method %s", method.MethodName)
			}
		}
	}
	return nil
}
