// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"testing"

	"chromiumos/tast/errors"
)

func TestExpectIface(t *testing.T) {
	for _, p := range []struct {
		es  []string
		as  []string
		err error
	}{
		{[]string{"eth0", "wlan0"}, []string{"eth0", "wlan0"}, nil},
		{[]string{"eth0", "wlan0"}, []string{"eth1", "wlan0"}, errors.New("failed expecting network interfaces: wanted:[eth0], unexpected:[eth1], matched:[wlan0]")},
		{[]string{"eth0", "wlan0"}, []string{"eth0", "arc_eth0", "wlan0"}, errors.New("failed expecting network interfaces: unexpected:[arc_eth0], matched:[eth0 wlan0]")},
		{[]string{"eth0", "wlan0"}, []string{"eth0"}, errors.New("failed expecting network interfaces: wanted:[wlan0], matched:[eth0]")},
	} {
		e := expectIface(p.es, p.as)
		if p.err == nil && e == nil {
			continue
		}
		if p.err == nil {
			t.Errorf("expectIface(%s, %s) expects no error but it returns one: %q", p.es, p.as, e)
		} else if e == nil {
			t.Errorf("expectIface(%s, %s) fails to return the expected error: %q", p.es, p.as, p.err)
		} else if e.Error() != p.err.Error() {
			t.Errorf("expectIface(%s, %s) wrongly returns an error with message: %q, expect %q", p.es, p.as, e, p.err)
		}
	}
}
