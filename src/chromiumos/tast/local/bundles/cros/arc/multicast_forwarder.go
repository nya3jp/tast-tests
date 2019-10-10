// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sync/errgroup"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MulticastForwarder,
		Desc:         "Checks if multicast forwarder works on ARC++",
		Contacts:     []string{"jasongustaman@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcMulticastForwarderTest.apk"},
		Pre:          arc.Booted(),
	})
}

func MulticastForwarder(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcMulticastForwarderTest.apk"
		pkg = "org.chromium.arc.testapp.multicast_forwarder"
		cls = "org.chromium.arc.testapp.multicast_forwarder.MulticastForwarderActivity"

		mdnsButtonID   = "org.chromium.arc.testapp.multicast_forwarder:id/button_mdns"
		ssdpButtonID   = "org.chromium.arc.testapp.multicast_forwarder:id/button_ssdp"
		dataID         = "org.chromium.arc.testapp.multicast_forwarder:id/data"
		portID         = "org.chromium.arc.testapp.multicast_forwarder:id/port"
		ipv6CheckboxID = "org.chromium.arc.testapp.multicast_forwarder:id/checkbox_ipv6"

		// Random inbound hostnames and user agents used to verify if packet successfully forwarded.
		mdnsHostnameIn           = "bff40af49dd97a3d1951f5af9c2b648099f15bbb.local"
		mdnsHostnameInIPv6       = "da39a3ee5e6b4b0d3255bfef95601890afd80709.local"
		legacyMdnsHostnameIn     = "a2ffec2cb85be5e7be43b2b0f4b7187379347e27.local"
		legacyMdnsHostnameInIPv6 = "430c8b722af2d577a3d689ca6d82821385442907.local"
		ssdpUserAgentIn          = "3d4a9db69bb32af3631825c556840656138cea3c"
		ssdpUserAgentInIPv6      = "600c5082adf6270c25649bcdecf0584f203b7cb7"

		// Random outbound hostnames and user agents used to verify if packet successfully forwarded.
		mdnsHostnameOut           = "011f4b05bbe073f2ee322356159daa0a3ea5793f.local"
		mdnsHostnameOutIPv6       = "bed4eb698c6eeea7f1ddf5397d480d3f2c0fb938.local"
		legacyMdnsHostnameOut     = "6c0596b8ac609191181a90517d51c0b486f23799.local"
		legacyMdnsHostnameOutIPv6 = "ce15802a8c5e8e9db0ffaf10130ef265296e9ea4.local"
		ssdpUserAgentOut          = "e81a11db0ca4a137276eca2f189279f038219a23"
		ssdpUserAgentOutIPv6      = "d03754dadcd065da9063f9fb6c392e9f66880830"

		// These ports are used as source addresses to send packet.
		mdnsPort       = "5353"
		legacyMdnsPort = "10101"
		ssdpPort       = "9191"

		// These prefixes are used to search the correct packets from tcpdump.
		ssdpPrefix = "USER-AGENT: "
		mdnsPrefix = "(QM)? "

		// Timeout and retry constant.
		timeout = 90 * time.Second
		retry   = 3
	)

	ifnames, err := physicalInterfaces(ctx)
	if err != nil {
		s.Fatal("Failed to get physical interfaces: ", err)
	}

	// Check the physical interfaces for multicast support.
	// This is done by checking multicast flag followed by IPv4 and IPv6 existence.
	ipv4Multicast, ipv6Multicast := false, false
	for _, ifname := range ifnames {
		iface, err := net.InterfaceByName(ifname)
		if err != nil {
			s.Fatal("Failed to get interface by name: ", err)
		}

		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			s.Fatal("Failed to get interface addresses: ", err)
		}

		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				s.Fatal("Failed to parse interface CIDR: ", err)
			}
			if ip.To4() != nil {
				ipv4Multicast = true
				continue
			}
			if ip.To16() != nil {
				ipv6Multicast = true
			}
		}
	}

	// Start ARC multicast sender app.
	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	if err := d.Object(ui.ID(mdnsButtonID)).WaitForExists(ctx, timeout); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	// expectOut and expectIn hold expected strings to be found in the tcpdump stream
	// for outbound and inbound test respectively.
	expectOut := make(map[string]struct{})
	expectIn := make(map[string]struct{})

	// Adds expectations for tcpdump.
	if ipv4Multicast {
		expectOut[mdnsPrefix+mdnsHostnameOut] = struct{}{}
		expectOut[mdnsPrefix+legacyMdnsHostnameOut] = struct{}{}
		expectOut[ssdpPrefix+ssdpUserAgentOut] = struct{}{}
		expectIn[mdnsPrefix+mdnsHostnameIn] = struct{}{}
		expectIn[mdnsPrefix+legacyMdnsHostnameIn] = struct{}{}
		expectIn[ssdpPrefix+ssdpUserAgentIn] = struct{}{}
	}
	if ipv6Multicast {
		expectOut[mdnsPrefix+mdnsHostnameOutIPv6] = struct{}{}
		expectOut[mdnsPrefix+legacyMdnsHostnameOutIPv6] = struct{}{}
		expectOut[ssdpPrefix+ssdpUserAgentOutIPv6] = struct{}{}
		expectIn[mdnsPrefix+mdnsHostnameInIPv6] = struct{}{}
		expectIn[mdnsPrefix+legacyMdnsHostnameInIPv6] = struct{}{}
		expectIn[ssdpPrefix+ssdpUserAgentInIPv6] = struct{}{}
	}

	s.Log("Starting tcpdump")
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		tcpdump := testexec.CommandContext(ctx, "/usr/local/sbin/tcpdump", "-Ani", "any", "-Q", "out")
		if err := streamCmd(ctx, tcpdump, expectOut); err != nil {
			return errors.Errorf("outbound test failed to get packet %s %s", expectOut, err)
		}
		return nil
	})
	g.Go(func() error {
		tcpdump := arc.BootstrapCommand(ctx, "/system/xbin/tcpdump", "-Ani", "any", "-Q", "in")
		if err := streamCmd(ctx, tcpdump, expectIn); err != nil {
			return errors.Errorf("inbound test failed to get packet %s %s", expectIn, err)
		}
		return nil
	})

	// setTexts edit multicast sender parameter by setting EditTexts' text.
	// This function shouldn't be called concurrently as it is closes over |d|.
	setTexts := func(hostname, port string) {
		if err := d.Object(ui.ID(dataID)).SetText(ctx, hostname); err != nil {
			s.Error("Failed setting hostname: ")
		}
		if err := d.Object(ui.ID(dataID), ui.Focused(true)).WaitForExists(ctx, timeout); err != nil {
			s.Errorf("Failed to focus on field %s", dataID)
		}

		if err := d.Object(ui.ID(portID)).SetText(ctx, port); err != nil {
			s.Error("Failed setting port: ", err)
		}
		if err := d.Object(ui.ID(portID), ui.Focused(true)).WaitForExists(ctx, timeout); err != nil {
			s.Errorf("Failed to focus on field %s", portID)
		}
	}

	toggleIPv6 := func() {
		if err := d.Object(ui.ID(ipv6CheckboxID)).Click(ctx); err != nil {
			s.Error("Failed to toggle IPv6 checkbox: ", err)
		}
	}

	s.Log("Sending multicast packets")
	// Try to send multicast packet multiple times.
	for i := 0; i < retry; i++ {
		// Send outbound multicast packets from ARC.
		if ipv4Multicast {
			// Run mDNS query.
			setTexts(mdnsHostnameOut, mdnsPort)
			if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound mDNS test: ", err)
			}
			// Run legacy mDNS query.
			setTexts(legacyMdnsHostnameOut, legacyMdnsPort)
			if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound legacy mDNS test: ", err)
			}
			// Run SSDP query
			setTexts(ssdpUserAgentOut, ssdpPort)
			if err := d.Object(ui.ID(ssdpButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound SSDP test: ", err)
			}
		}
		if ipv6Multicast {
			toggleIPv6()
			// Run IPv6 mDNS query.
			setTexts(mdnsHostnameOutIPv6, mdnsPort)
			if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound IPv6 mDNS test: ", err)
			}
			// Run IPv6 legacy mDNS query.
			setTexts(legacyMdnsHostnameOutIPv6, legacyMdnsPort)
			if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound IPv6 legacy mDNS test: ", err)
			}
			// Run IPv6 SSDP query
			setTexts(ssdpUserAgentOutIPv6, ssdpPort)
			if err := d.Object(ui.ID(ssdpButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound IPv6 SSDP test: ", err)
			}
			toggleIPv6()
		}

		// Send inbound multicast packets by sending multicast packet that loops back.
		for _, ifname := range ifnames {
			if ipv4Multicast {
				// Run mDNS query.
				if err := sendMdns(mdnsHostnameIn, mdnsPort, ifname, false); err != nil {
					s.Error("Failed starting inbound mDNS test: ", err)
				}
				// Run legacy mDNS query.
				if err := sendMdns(legacyMdnsHostnameIn, legacyMdnsPort, ifname, false); err != nil {
					s.Error("Failed starting inbound legacy mDNS test: ", err)
				}
				// Run SSDP query
				if err := sendSsdp(ssdpUserAgentIn, ssdpPort, ifname, false); err != nil {
					s.Error("Failed starting inbound SSDP test: ", err)
				}
			}
			if ipv6Multicast {
				// Run IPv6 mDNS query.
				if err := sendMdns(mdnsHostnameInIPv6, mdnsPort, ifname, true); err != nil {
					s.Error("Failed starting inbound IPv6 mDNS test: ", err)
				}
				// Run IPv6 legacy mDNS query.
				if err := sendMdns(legacyMdnsHostnameInIPv6, legacyMdnsPort, ifname, true); err != nil {
					s.Error("Failed starting inbound IPv6 legacy mDNS test: ", err)
				}
				// Run IPv6 SSDP query
				if err := sendSsdp(ssdpUserAgentInIPv6, ssdpPort, ifname, true); err != nil {
					s.Error("Failed starting inbound IPv6 SSDP test: ", err)
				}
			}
		}
	}

	if err := g.Wait(); err != nil {
		s.Fatal("Failed multicast forwarding check: ", err)
	}
}

func physicalInterfaces(ctx context.Context) ([]string, error) {
	out, err := testexec.CommandContext(ctx, "/usr/bin/find", "/sys/class/net", "-type", "l", "-not", "-lname", "*virtual*", "-printf", "%f\n").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get physical interfaces")
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}

// sendMdns creates an mDNS question query for |hostname| with a socket bound to |port| and |ifname|.
// This will call sendMulticast which intentionally set a flag to loopback the multicast packet.
func sendMdns(hostname, port, ifname string, IPv6 bool) error {
	// Craft mDNS message.
	b := bytes.NewBuffer(nil)
	// Add header to buffer.
	b.Write([]byte{0x0, 0x0})           // Transaction ID = 0
	b.Write([]byte{0x0, 0x0})           // Flags = 0
	b.Write([]byte{0x0, 0x1})           // Number of questions = 1
	b.Write([]byte{0x0, 0x0})           // Number of answers = 1
	b.Write([]byte{0x0, 0x0, 0x0, 0x0}) // Number of resource records = 0
	// Add hostname to buffer
	for _, data := range strings.Split(hostname, ".") {
		b.WriteByte(uint8(len(data)))
		b.Write([]byte(data))
	}
	b.WriteByte(0x0)          // Terminator
	b.Write([]byte{0x0, 0x1}) // QTYPE = A record
	b.Write([]byte{0x0, 0x1}) // QTYPE = IN class

	p, err := strconv.Atoi(port)
	if err != nil {
		return errors.Errorf("failed to convert port to int %s", err)
	}

	var dst *net.UDPAddr
	if IPv6 {
		dst = &net.UDPAddr{IP: net.ParseIP("ff02::fb"), Port: 5353}
	} else {
		dst = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	}

	if err := sendMulticast(b.Bytes(), p, dst, ifname, IPv6); err != nil {
		return err
	}
	return nil
}

// sendSsdp creates an SSDP search query with USER-AGENT |ua| with a socket bound to |port| and |ifname|.
// This will call sendMulticast which intentionally set a flag to loopback the multicast packet.
func sendSsdp(ua, port, ifname string, IPv6 bool) error {
	// Craft SSDP message.
	b := bytes.NewBuffer(nil)
	b.Write([]byte("M-SEARCH * HTTP/1.1\r\n"))
	b.Write([]byte("HOST: 239.255.255.250:1900\r\n"))
	b.Write([]byte("MAN: \"ssdp:discover\"\r\n"))
	b.Write([]byte("MX: 3\r\n"))
	b.Write([]byte("ST: ssdp:all\r\n"))
	b.Write([]byte("USER-AGENT: " + ua + "\r\n\r\n"))

	p, err := strconv.Atoi(port)
	if err != nil {
		return errors.Errorf("failed to convert port to int %s", err)
	}

	var dst *net.UDPAddr
	if IPv6 {
		dst = &net.UDPAddr{IP: net.ParseIP("ff02::c"), Port: 1900}
	} else {
		dst = &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	}

	if err := sendMulticast(b.Bytes(), p, dst, ifname, IPv6); err != nil {
		return err
	}
	return nil
}

// sendMulticast take data |b| and send it using a temporarily create socket bound to |port| and |ifname|.
// This function intentionally set multicast loopback to true.
func sendMulticast(b []byte, port int, dst *net.UDPAddr, ifname string, IPv6 bool) error {
	var d int
	if IPv6 {
		d = syscall.AF_INET6
	} else {
		d = syscall.AF_INET
	}

	s, err := syscall.Socket(d, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return errors.Errorf("failed to open socket %s", err)
	}
	defer syscall.Close(s)

	if err := syscall.SetsockoptInt(s, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return errors.Errorf("failed setsockopt(SO_REUSEADDR) %s", err)
	}
	if err := syscall.SetsockoptString(s, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, ifname); err != nil {
		return errors.Errorf("failed setsockopt(SO_BINDTODEVICE) %s", err)
	}

	var sa syscall.Sockaddr
	if IPv6 {
		sa = &syscall.SockaddrInet6{Port: port}
	} else {
		sa = &syscall.SockaddrInet4{Port: port}
	}

	if err := syscall.Bind(s, sa); err != nil {
		return errors.Errorf("failed bind() %s", err)
	}

	// Create a PacketConn from a copy of recently created socket |s|.
	f := os.NewFile(uintptr(s), "")
	c, err := net.FilePacketConn(f)
	if err != nil {
		return errors.Errorf("failed to create packet conn %s", err)
	}
	defer c.Close()

	if IPv6 {
		pc := ipv6.NewPacketConn(c)
		if err := pc.SetMulticastLoopback(true); err != nil {
			return errors.Errorf("failed to set multicast loopback %s", err)
		}
	} else {

		pc := ipv4.NewPacketConn(c)
		if err := pc.SetMulticastLoopback(true); err != nil {
			return errors.Errorf("failed to set multicast loopback %s", err)
		}
	}

	c.WriteTo(b, dst)
	return nil
}

// streamCmd takes a command |cmd| and stream its output. It search its output for every string in set |s|.
// This function will return an error if all string in |s| is not found before context is finished.
func streamCmd(ctx context.Context, cmd *testexec.Cmd, s map[string]struct{}) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start command")
	}

	// sc.Scan() below might block. Release bufio.Scanner by killing command if the
	// process execution time exceeds context deadline.
	go func() {
		defer cmd.Wait()
		defer cmd.Kill()

		// Blocks until deadline is passed.
		<-ctx.Done()
	}()

	// Watch and wait until command have the expected outputs.
	sc := bufio.NewScanner(stdout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if !sc.Scan() {
			continue
		}

		t := sc.Text()
		for a := range s {
			if strings.Contains(t, a) {
				delete(s, a)
			}
		}

		if len(s) == 0 {
			return nil
		}
	}
}
