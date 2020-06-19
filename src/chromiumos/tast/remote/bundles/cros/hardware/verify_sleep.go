// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/testing"
)

const (
	sleepDuration   = 3 * time.Second
	sleepIterations = 2
	sleepTolerance  = 20 * time.Millisecond
	socketLevel     = syscall.IPPROTO_TCP
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyRemoteSleep,
		Desc:         "Verifies that sleeps on DUT are as long as they should be",
		Contacts:     []string{"semihalf@google.com"},
		Attr:         []string{},
		Timeout:      sleepDuration*sleepIterations + time.Minute,
		SoftwareDeps: []string{"remote_sleep_test"},
	})
}

// getTestingMachineIPs iterates over all interfaces and stores
// corresponding IPv4 addresses
func getTestingMachineIPs(s *testing.State) []net.IP {
	var result []net.IP

	ifaces, err := net.Interfaces()
	if err != nil {
		s.Fatal("Couldn't obtain interfaces: ", err)
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			s.Fatal("Failed to get ip addresses: ", err)
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			ip4 := ip.To4()
			if ip4 != nil && !ip4.IsLoopback() {
				result = append(result, ip4)
			}
		}
	}

	return result
}

// getDUTIPs uses SSH to query the DUT about all of its IPv4 addresses
func getDUTIPs(ctx context.Context, s *testing.State) []net.IP {
	command := "ifconfig | sed -En 's/127.0.0.1//;s/.*inet (addr:)?(([0-9]*\\.){3}[0-9]*).*/\\2/p'"
	var result []net.IP

	dut := s.DUT()
	cmd := dut.Conn().Command("sh", "-c", command)
	out, err := cmd.Output(ctx)

	if err != nil {
		s.Fatal("Couldn't fetch remote ip addresses: ", err)
	}

	lines := strings.Split(string(out), "\n")
	// there's an empty entry at the end because of the trailing newline
	ipStrings := lines[:len(lines)-1]
	for _, ip := range ipStrings {
		result = append(result, net.ParseIP(ip))
	}

	return result
}

// getTestingMachineIP finds an IP address on the testing machine
// that is visible to the DUT
func getTestingMachineIP(ctx context.Context, s *testing.State) net.IP {
	mask := net.IPv4Mask(255, 255, 0, 0)
	remote := getTestingMachineIPs(s)
	local := getDUTIPs(ctx, s)

	// check if any pair of IP addresses is in the same network
	for _, rIP := range remote {
		for _, lIP := range local {
			if rIP.Mask(mask).Equal(lIP.Mask(mask)) {
				return rIP
			}
		}
	}

	s.Fatal("No compatibile IP addresses")
	return nil
}

func doRemoteSleep(ctx context.Context, s *testing.State, d time.Duration, iters int64, ip net.IP, port int) {
	sleepArg := strconv.FormatInt(int64(d.Milliseconds()), 10)
	itersArg := strconv.FormatInt(iters, 10)
	command := fmt.Sprintf("sleep 1; remote_sleep_test %s %s %v %v", sleepArg, itersArg, ip, port)

	dut := s.DUT()
	cmd := dut.Conn().Command("sh", "-c", command)
	err := cmd.Start(ctx)

	if err != nil {
		s.Fatal("Couldn't start the remote sleep: ", err)
	}
}

// remoteSleepListenTCP randomizes TCP ports until listening is successful (up to 20 times)
// in case there are several instances of the test running concurrently
func remoteSleepListenTCP(s *testing.State, ip net.IP) (*net.TCPListener, int) {
	tries := 20
	for i := 0; i < tries; i++ {
		port := rand.Intn(65535-1) + 1

		addr := net.TCPAddr{
			Port: port,
			IP:   ip,
		}

		ser, err := net.ListenTCP("tcp", &addr)
		if err == nil {
			s.Log("Listening at ", addr)
			return ser, port
		}
	}

	s.Fatalf("Failed to start listening (tried %v random ports)", tries)
	return nil, 0
}

func measureRemoteSleep(ctx context.Context, s *testing.State, ip net.IP) []time.Duration {
	var durations []time.Duration
	p := make([]byte, 2048)

	ser, port := remoteSleepListenTCP(s, ip)
	doRemoteSleep(ctx, s, sleepDuration, sleepIterations, ip, port)

	sockFile, err := ser.File()
	if err != nil {
		s.Fatal("Couldn't retrieve underlying file from a TCP socket: ", err)
	}

	socketDesc := int(sockFile.Fd())
	err = syscall.SetsockoptInt(socketDesc, socketLevel, syscall.TCP_QUICKACK, 1)
	if err != nil {
		s.Fatal("Enabling TCP_QUICKACK failed: ", err)
	}

	conn, err := ser.Accept()
	if err != nil {
		s.Fatal("Accepting connection failed: ", err)
	}

	_, err = conn.Read(p)
	if err != nil {
		s.Fatal("Reading from connection failed: ", err)
	}

	// the datapoints are the delays between consecutive received packets
	for {
		start := time.Now()

		_, err2 := conn.Read(p)
		if err2 != nil {
			s.Log("Session broken")
			break
		}

		elapsed := time.Since(start)
		durations = append(durations, elapsed)
		measuredMs := float32(elapsed) / float32(time.Millisecond)
		s.Logf("Measured: %vms", measuredMs)
	}

	return durations
}

func VerifyRemoteSleep(ctx context.Context, s *testing.State) {
	ip := getTestingMachineIP(ctx, s)

	measured := measureRemoteSleep(ctx, s, ip)
	toleranceAdjusted := sleepDuration - sleepTolerance
	failed := false

	s.Logf("Min valid sleep time: %vms", toleranceAdjusted.Milliseconds())

	for _, dur := range measured {
		// measuredMs smaller than requested always implies an error,
		// network delays can't be negative
		if dur < toleranceAdjusted {
			failed = true
			s.Logf("[ERR] %vms < %vms", dur.Milliseconds(), sleepDuration.Milliseconds())
		}
	}

	if failed {
		s.Fatalf("Some measured sleeps were shorter than %vms", sleepDuration.Milliseconds())
	}
}
