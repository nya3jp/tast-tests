// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        POC,
		Desc:        "Template for creating PoC tests",
		Contacts:    []string{"yenlinlai@google.com"},
		Attr:        []string{},
		ServiceDeps: []string{"tast.cros.network.Log"},
		Pre:         wificell.TestLogPre(),
		Params: []testing.Param{
			{
				Name: "t1",
				Val:  5,
			},
			{
				Name: "t2",
				Val:  5,
			},
			{
				Name: "t3",
				Val:  5,
			},
		},
	})
}

func POC(ctx context.Context, s *testing.State) {
	cli := s.PreValue().(network.LogClient)
	c := s.Param().(int)
	_, err := cli.Print(ctx, &network.String{S: "start"})
	if err != nil {
		s.Fatal("Print failed: ", err)
	}
	for i := 0; i < c; i++ {
		_, err := cli.Print(ctx, &network.String{S: fmt.Sprintf("loop %d: %v", i, time.Now())})
		if err != nil {
			s.Fatal("Print failed: ", err)
		}
	}
}
