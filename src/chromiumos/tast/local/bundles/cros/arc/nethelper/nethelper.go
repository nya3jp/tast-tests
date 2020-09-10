// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nethelper provides functionality to support test execution by handling
// requests from various tests coming via network in context of ARC TAST test.
// arc_eth0 on port 1235 is used as communication point. This helper currently
// supports the following commands:
//   * drop_caches - drops system caches, returns OK/FAILED
//
// Usage pattern is following:
// 	conns, err := nethelper.Start(ctx)
//	if err != nil {
//		s.Fatal("Failed to start helper", err)
//	}
//	defer nethelper.Stop(ctx, nethelperPort)
// for _, conn := range conns {
// 	defer conn.Close()
// }
package nethelper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"

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

// Connection describes running socket server.
type Connection struct {
	// Contains network address that could be used by client to connect this server
	Address  string
	listener net.Listener
}

// Close closes the connection
func (c *Connection) Close() {
	c.listener.Close()
}

// Start starts socket server and returns connection descriptor.
func Start(ctx context.Context, port int) ([]*Connection, error) {
	cmds := []string{iptablesCmd, ip6tablesCmd}
	ifacesArc := []string{"arc_eth+", "arc_mlan+", "arc_wlan+", "arcbr+"}

	// Make sure ports may accepts connection.
	for _, cmd := range cmds {
		for _, ifaceArc := range ifacesArc {
			if err := testexec.CommandContext(ctx,
				cmd, "-w", "-A", "INPUT", "-i", ifaceArc, "-p", "tcp",
				"--dport", strconv.Itoa(port),
				"-j", "ACCEPT").Run(testexec.DumpLogOnError); err != nil {
				return nil, errors.Wrapf(err, "failed to open port %d", port)
			}

			rule := "-A INPUT -i " + ifaceArc + " -p tcp -m tcp --dport " + strconv.Itoa(port) + " -j ACCEPT"

			// Check rules were added in IPv4 and IPv6 iptables.
			if cmd == iptablesCmd {
				rules, err := iptablesRules(ctx, iptablesCmd)
				if err != nil {
					return nil, errors.Wrap(err, "failed obtain iptables rules")
				}
				if err := iptablesCheck(rule, rules, true); err != nil {
					return nil, errors.Wrap(err, "failed to add IPv4 rule")
				}
			}
			if cmd == ip6tablesCmd {
				rules6, err := iptablesRules(ctx, ip6tablesCmd)
				if err != nil {
					return nil, errors.Wrap(err, "failed obtain ip6tables rules")
				}
				if err := iptablesCheck(rule, rules6, true); err != nil {
					return nil, errors.Wrap(err, "failed to add IPv6 rule")
				}
			}
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.Wrap(err, "failed to enum interfaces")
	}

	var connections []*Connection
	for _, i := range ifaces {
		// Handle real addresses that could be accessible for the current DUT.
		if !strings.HasPrefix(i.Name, "arc") {
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
				result := new(Connection)
				if v.IP.To4() != nil {
					result.Address = fmt.Sprintf("%s:%d", v.IP, port)
				} else {
					result.Address = fmt.Sprintf("[%s%%%s]:%d", v.IP, i.Name, port)
				}
				result.listener, err = net.Listen("tcp", result.Address)
				if err != nil {
					testing.ContextLogf(ctx, "Failed to listen on %s", result.Address)
					continue
				}
				testing.ContextLogf(ctx, "Listening on %s", result.Address)
				go listenForClients(ctx, result.listener)
				connections = append(connections, result)
			}
		}
	}

	if len(connections) > 0 {
		return connections, nil
	}

	return nil, errors.Errorf("failed to start server at port %d, no address is availables", port)
}

// Stop deletes iptables input rules.
func Stop(ctx context.Context, port int) error {
	cmds := []string{iptablesCmd, ip6tablesCmd}
	ifacesArc := []string{"arc_eth+", "arc_mlan+", "arc_wlan+", "arcbr+"}

	// Delete iptables rules that were created.
	for _, cmd := range cmds {
		for _, ifaceArc := range ifacesArc {
			if err := testexec.CommandContext(ctx,
				cmd, "-w", "-D", "INPUT", "-i", ifaceArc, "-p", "tcp",
				"--dport", strconv.Itoa(port),
				"-j", "ACCEPT").Run(testexec.DumpLogOnError); err != nil {
				return errors.Wrapf(err, "failed to close port %d", port)
			}

			rule := "-A INPUT -i " + ifaceArc + " -p tcp -m tcp --dport " + strconv.Itoa(port) + " -j ACCEPT"

			// Check the previously created iptables rules were successfully removed.
			if cmd == iptablesCmd {
				rules, err := iptablesRules(ctx, iptablesCmd)
				if err != nil {
					return errors.Wrap(err, "failed obtain iptables rules")
				}
				if err := iptablesCheck(rule, rules, false); err != nil {
					return errors.Wrap(err, "failed to remove IPv4 rule")
				}
			}
			if cmd == ip6tablesCmd {
				rules6, err := iptablesRules(ctx, ip6tablesCmd)
				if err != nil {
					return errors.Wrap(err, "failed obtain ip6tables rules")
				}
				if err := iptablesCheck(rule, rules6, false); err != nil {
					return errors.Wrap(err, "failed to remove IPv6 rule")
				}
			}
		}
	}

	return nil
}

func listenForClients(ctx context.Context, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			testing.ContextLogf(ctx, "Stop listening %s", err)
			return
		}
		testing.ContextLogf(ctx, "Connection is ready %s", conn.RemoteAddr().String())
		go handleClient(ctx, conn)
	}

}

func handleClient(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	const (
		cmdDropCaches = "drop_caches"
	)

	for {
		message, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			if err == io.EOF {
				testing.ContextLogf(ctx, "Connection is closed %s", conn.RemoteAddr().String())
				return
			}
			testing.ContextLogf(ctx, "Connection is broken %s: %s", conn.RemoteAddr().String(), err)
			return
		}
		msg := strings.TrimSuffix(string(message), "\n")
		switch msg {
		case cmdDropCaches:
			result := handleDropCaches(ctx)
			_, err := conn.Write([]byte(result + "\n"))
			if err != nil {
				testing.ContextLogf(ctx, "Failed to respond %q to %s, %s", result, conn.RemoteAddr().String(), err)
				return
			}
		default:
			testing.ContextLogf(ctx, "Unknown command %s: %s", conn.RemoteAddr().String(), msg)
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

func iptablesRules(ctx context.Context, cmd string) ([]string, error) {
	out, err := testexec.CommandContext(ctx, cmd, "-S").Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get iptables rules with: %s", cmd)
	}

	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}

func iptablesCheck(query string, rules []string, expected bool) error {
	found := false
	for _, rule := range rules {
		if rule == query {
			found = true
			break
		}
	}

	if found != expected {
		return errors.New("failed to add iptables rule")
	}

	return nil
}
