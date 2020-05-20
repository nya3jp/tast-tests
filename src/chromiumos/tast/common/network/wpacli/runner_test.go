// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpacli

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"chromiumos/tast/errors"
)

func TestSudoWpacli(t *testing.T) {
	testcases := []struct {
		input  []string
		expect []string
	}{
		{
			input:  nil,
			expect: []string{"-u", "wpa", "-g", "wpa", "wpa_cli"},
		},
		{
			input:  []string{"-i", "wlan0", "ping"},
			expect: []string{"-u", "wpa", "-g", "wpa", "wpa_cli", "-i", "wlan0", "ping"},
		},
	}
	for _, tc := range testcases {
		result := sudoWpacli(tc.input...)
		if !reflect.DeepEqual(result, tc.expect) {
			t.Errorf("sudoWpacli outputs differs; got %v, want %v", result, tc.expect)
		}
	}
}

type cmdOutError struct {
	cmdOut []byte
	err    error
}

type cmdRunner struct {
	script map[string]cmdOutError
}

func newCmdRunner() *cmdRunner {
	return &cmdRunner{script: make(map[string]cmdOutError)}
}

func (r *cmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if len(args) < 2 {
		return nil, errors.Errorf("insufficent #args: got %d; want >=2", len(args))
	}
	iface := args[len(args)-2]
	s, ok := r.script[iface]
	if !ok {
		return nil, errors.Errorf("invalid interface %s", iface)
	}
	return s.cmdOut, s.err
}

func (r *cmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	return errors.New("shall not be called")
}

func TestPing(t *testing.T) {
	cr := newCmdRunner()
	cr.script["wlan0"] = cmdOutError{cmdOut: []byte("PONG"), err: nil}
	cr.script["wlan1"] = cmdOutError{cmdOut: []byte("..."), err: nil}

	r := NewRunner(cr)
	ctx := context.Background()

	testcases := []struct {
		iface  string
		expect cmdOutError
	}{
		{
			iface: "foo",
			expect: cmdOutError{
				cmdOut: nil,
				err:    errors.New("failed running wpa_cli"),
			},
		},
		{
			iface: "wlan0",
			expect: cmdOutError{
				cmdOut: []byte("PONG"),
				err:    nil,
			},
		},
		{
			iface: "wlan1",
			expect: cmdOutError{
				cmdOut: []byte("..."),
				err:    errors.New("failed to see 'PONG'"),
			},
		},
	}

	// hasPrefix returns true if err begins with expect.
	hasPrefix := func(err, expect error) bool {
		return strings.HasPrefix(err.Error(), expect.Error())
	}

	for _, tc := range testcases {
		cmdOut, err := r.Ping(ctx, tc.iface)
		if tc.expect.cmdOut == nil {
			if cmdOut != nil {
				t.Errorf("Unexpected Ping(%s) output: got %s, want nil", tc.iface, string(cmdOut))
			}
		} else {
			if !bytes.Equal(cmdOut, tc.expect.cmdOut) {
				t.Errorf("Unexpected Ping(%s) output: got %s, want %s", tc.iface, string(cmdOut), string(tc.expect.cmdOut))
			}
		}
		if tc.expect.err == nil {
			if err != nil {
				t.Errorf("Unexpected Ping(%s) err: got %s, want nil", tc.iface, err)
			}
		} else {
			if !hasPrefix(err, tc.expect.err) {
				t.Errorf("Unexpected Ping(%s) err: got %s, want prefix %s", tc.iface, err, tc.expect.err)
			}
		}
	}

}
