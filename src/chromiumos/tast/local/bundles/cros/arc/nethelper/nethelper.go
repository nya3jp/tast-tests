// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nethelper provides functionality to support test execution by handling
// requests from various tests coming via network in context of ARC TAST test.
// arc_eth0 on port 1235 is used as communication point. This helper currently
// supports the following commands:
//   * drop_caches - drops system caches, returns OK/FAILED
//   * receive_payload - receives payload from client, return OK/FAILED and ACK
//
// Usage pattern is following:
// 	conn, err := nethelper.Start(ctx)
//	if err != nil {
//		s.Fatal("Failed to start nethelper", err)
//	}
//	defer conn.Close(ctx)
package nethelper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	okResponse     = "OK"
	failedResponse = "FAILED"

	iptablesCmd  = "/sbin/iptables"
	ip6tablesCmd = "/sbin/ip6tables"
)

var (
	cmds      = []string{iptablesCmd, ip6tablesCmd}
	ifacesArc = []string{"arc_eth+", "arc_mlan+", "arc_wlan+", "arcbr+"}
)

// Connection describes running socket server context.
type Connection struct {
	// Contains network listeners that could be used by client to connect this server.
	listeners []net.Listener
	rules     []string
}

// Close cleans up the connection descriptor.
func (c *Connection) Close(ctx context.Context) error {
	var firstErr error
	for _, listener := range c.listeners {
		if err := listener.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Delete iptables rules that were created.
	for _, rule := range c.rules {
		for _, cmd := range cmds {
			args := append([]string{"-D"}, strings.Fields(rule)...)
			if err := testexec.CommandContext(ctx, cmd, args...).Run(testexec.DumpLogOnError); err != nil && firstErr == nil {
				firstErr = err
			}

			// Check the previously created iptables rules were successfully removed.
			args = append([]string{"-C"}, strings.Fields(rule)...)
			if err := testexec.CommandContext(ctx, cmd, args...).Run(); err == nil && firstErr == nil {
				firstErr = errors.Errorf("failed to verify removal of iptables rule: %s", rule)
			}
		}
	}

	// Only return the first error as it's usually the most interesting one.
	return firstErr
}

// Start starts socket server and returns connection descriptor.
func Start(ctx context.Context, port int) (*Connection, error) {
	result := new(Connection)
	ifacesArcSet := make(map[string]bool)

	// Make sure ARC interfaces can accept connection on port
	for _, i := range ifacesArc {
		// For faster search when checking existing net interfaces.
		key := strings.TrimSuffix(i, "+")
		if _, ok := ifacesArcSet[key]; !ok {
			ifacesArcSet[key] = true
		}

		rule := "INPUT -i " + i + " -p tcp -m tcp --dport " + strconv.Itoa(port) + " -j ACCEPT -w"
		result.rules = append(result.rules, rule)
		for _, cmd := range cmds {
			args := append([]string{"-A"}, strings.Fields(rule)...)
			if err := testexec.CommandContext(ctx, cmd, args...).Run(testexec.DumpLogOnError); err != nil {
				return nil, errors.Wrapf(err, "failed to add iptables rule: %s", rule)
			}

			// Check rules were added in IPv4 and IPv6 iptables.
			args = append([]string{"-C"}, strings.Fields(rule)...)
			if err := testexec.CommandContext(ctx, cmd, args...).Run(); err != nil {
				return nil, errors.Wrapf(err, "failed to verify addition of iptables rule: %s", rule)
			}
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.Wrap(err, "failed to enum interfaces")
	}

	// Handle real addresses that could be accessible for the current DUT.
	for _, i := range ifaces {
		// Filter network interfaces using ifacesArc and match non-wildcard lowercase characters.
		if _, ok := ifacesArcSet[regexp.MustCompile("[^a-z]+(_?[a-z]+)*$").ReplaceAllString(i.Name, "")]; !ok {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			testing.ContextLogf(ctx, "Failed to enum addresses for %s", i.Name)
			continue
		}
		if len(addrs) == 0 {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				var address string
				if v.IP.To4() != nil {
					address = fmt.Sprintf("%s:%d", v.IP, port)
				} else {
					address = fmt.Sprintf("[%s%%%s]:%d", v.IP, i.Name, port)
				}
				listener, err := net.Listen("tcp", address)
				if err != nil {
					testing.ContextLogf(ctx, "Failed to listen on %s", address)
					continue
				}

				// Deploy listen goroutine and add new connection listener to result object.
				testing.ContextLogf(ctx, "Listening on %s", address)
				go listenForClients(ctx, listener)
				result.listeners = append(result.listeners, listener)
			}
		}
	}

	if len(result.listeners) > 0 {
		return result, nil
	}

	return nil, errors.Errorf("failed to start server at port %d, no address is available", port)
}

func listenForClients(ctx context.Context, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			testing.ContextLogf(ctx, "Stop listening %s", err)
			return
		}
		testing.ContextLogf(ctx, "Connection is ready %s<->%s", conn.LocalAddr().String(), conn.RemoteAddr().String())
		go handleClient(ctx, conn)
	}

}

func handleClient(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	const (
		// Supported commands (unrecognized ones are ignored).
		cmdDropCaches     = "drop_caches"
		cmdReceivePayload = "receive_payload"

		tReadWaitTimeout = 1 * time.Minute
	)

	// Set read timeout in case remote connection is lost.
	if err := conn.SetReadDeadline(time.Now().Add(tReadWaitTimeout)); err != nil {
		testing.ContextLog(ctx, "Failed to set read deadline")
		return
	}

	// Use single bufio.Reader instance for each connection.
	r := bufio.NewReader(conn)
	for {
		message, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				testing.ContextLogf(ctx, "Connection is closed %s", conn.RemoteAddr().String())
				return
			}
			testing.ContextLogf(ctx, "Connection is broken %s: %s", conn.RemoteAddr().String(), err)
			return
		}
		msg := strings.TrimSuffix(message, "\n")

		switch msg {
		case cmdDropCaches:
			result := handleDropCaches(ctx)
			_, err := conn.Write([]byte(result + "\n"))
			if err != nil {
				testing.ContextLogf(ctx, "Failed to respond %q to %s, %s", result, conn.RemoteAddr().String(), err)
				return
			}
		case cmdReceivePayload:
			if _, err := conn.Write([]byte(okResponse + "\n")); err != nil {
				testing.ContextLogf(ctx, "Failed to notify %s, %s", conn.RemoteAddr().String(), err)
				return
			}
			if result, err := handleReceivePayload(conn, r); err != nil {
				testing.ContextLogf(ctx, "Failed to receive payload from %s: %s", conn.RemoteAddr().String(), err)
				return
			} else if result > 0 {
				testing.ContextLogf(ctx, "Received %d bytes payload from %s", result, conn.RemoteAddr().String())
			}
		default:
			testing.ContextLogf(ctx, "Unknown command from %s: %s", conn.RemoteAddr().String(), msg)
		}
	}
}

func handleDropCaches(ctx context.Context) string {
	if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
		testing.ContextLogf(ctx, "Failed to flush buffers: %s", err)
		return failedResponse
	}
	if err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0200); err != nil {
		testing.ContextLogf(ctx, "Failed to drop caches: %s", err)
		return failedResponse
	}
	testing.ContextLog(ctx, "Flushed file system buffer, cleared caches, dentries and inodes")
	return okResponse
}

func handleReceivePayload(conn net.Conn, r *bufio.Reader) (int64, error) {
	ackMsg := fmt.Sprintf("Ack from nethelper connection %s pid=%s", conn.LocalAddr().String(), strconv.Itoa(os.Getpid()))

	// Read header containing size of the payload.
	message, err := r.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return 0, errors.New("failed to read header with EOF, connection is closed")
		}
		return 0, errors.Wrap(err, "failed to read header, connection is broken")
	}
	payloadSize, err := strconv.ParseInt(strings.TrimSuffix(message, "\n"), 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse header")
	}

	// Read payload and discard immediately since it's not used.
	if bytesRead, err := io.CopyN(ioutil.Discard, r, payloadSize); err != nil || bytesRead != payloadSize {
		return 0, errors.Wrap(err, "failed to read payload")
	}

	// Send Ack to complete transaction.
	if _, err := conn.Write([]byte(ackMsg)); err != nil {
		return 0, errors.Wrap(err, "failed to send ack message")
	}
	return payloadSize, nil
}
