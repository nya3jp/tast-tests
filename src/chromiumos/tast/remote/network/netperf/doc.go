// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package netperf is used for running performance tests using linux
// netperf/netserver suite.
// Example usage:
// netperfSession := netperf.NewSession(
// 	clientHost.conn,
// 	clientHost.IP, // DUT IP
// 	serverHost.conn,
// 	serverHost.IP) // Router IP
// defer func(ctx context.Context) {
// 	netperfSession.Close(ctx)
// }(ctx)
// ret, err = netperfSession.Run(ctx, netperf.Config{
// 	TestTime:         10*time.Second,
// 	TestType:         netperf.TestTypeTCPStream,
// 	HumanReadableTag: "TCP Stream"})
package netperf
