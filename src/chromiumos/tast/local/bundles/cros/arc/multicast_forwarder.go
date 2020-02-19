// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"
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
		Contacts:     []string{"jasongustaman@chromium.org", "cros-networking@google.com", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcMulticastForwarderTest.apk"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
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

		// Randomly generated inbound hostnames and user agents used to verify if packet successfully forwarded.
		mdnsHostnameIn           = "bff40af49dd97a3d1951f5af9c2b648099f15bbb.local"
		mdnsHostnameInIPv6       = "da39a3ee5e6b4b0d3255bfef95601890afd80709.local"
		legacyMDNSHostnameIn     = "a2ffec2cb85be5e7be43b2b0f4b7187379347e27.local"
		legacyMDNSHostnameInIPv6 = "430c8b722af2d577a3d689ca6d82821385442907.local"
		ssdpUserAgentIn          = "3d4a9db69bb32af3631825c556840656138cea3c"
		ssdpUserAgentInIPv6      = "600c5082adf6270c25649bcdecf0584f203b7cb7"

		// Randomly generated outbound hostnames and user agents used to verify if packet successfully forwarded.
		mdnsHostnameOut           = "011f4b05bbe073f2ee322356159daa0a3ea5793f.local"
		mdnsHostnameOutIPv6       = "bed4eb698c6eeea7f1ddf5397d480d3f2c0fb938.local"
		legacyMDNSHostnameOut     = "6c0596b8ac609191181a90517d51c0b486f23799.local"
		legacyMDNSHostnameOutIPv6 = "ce15802a8c5e8e9db0ffaf10130ef265296e9ea4.local"
		ssdpUserAgentOut          = "e81a11db0ca4a137276eca2f189279f038219a23"
		ssdpUserAgentOutIPv6      = "d03754dadcd065da9063f9fb6c392e9f66880830"

		// These ports are used as source ports to send packets.
		mdnsPort       = "5353"
		legacyMDNSPort = "10101"
		ssdpPort       = "9191"

		// These prefixes are used to search the correct packets from tcpdump.
		ssdpPrefix = "USER-AGENT: "
		mdnsPrefix = "(QM)? "
	)

	ifnames, err := physicalInterfaces(ctx)
	if err != nil {
		s.Fatal("Failed to get physical interfaces: ", err)
	}

	// Check the physical interfaces for multicast support.
	// This is done by checking multicast flag followed by IPv4 existence.
	// We don't check for IPv6 as kernel always provision an EUI 64 derived like local address in the fe80::/64 prefix.
	ipv4Multicast := false
	ipv6Multicast := false
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
				break
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

	if err := d.Object(ui.ID(mdnsButtonID)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	// expectOut and expectIn hold expected strings to be found in the tcpdump stream
	// for outbound and inbound test respectively.
	// The value contained in the map is used for error reporting.
	expectOut := make(map[string]string)
	expectIn := make(map[string]string)

	// Adds expectations for tcpdump.
	if ipv4Multicast {
		expectOut[mdnsPrefix+mdnsHostnameOut] = "IPv4 mDNS"
		expectOut[mdnsPrefix+legacyMDNSHostnameOut] = "IPv4 legacy mDNS"
		expectOut[ssdpPrefix+ssdpUserAgentOut] = "IPv4 SSDP"
		expectIn[mdnsPrefix+mdnsHostnameIn] = "IPv4 mDNS"
		expectIn[mdnsPrefix+legacyMDNSHostnameIn] = "IPv4 legacy mDNS"
		expectIn[ssdpPrefix+ssdpUserAgentIn] = "IPv4 SSDP"
	}
	// Always expect IPv6 multicast forwarding.
	expectOut[mdnsPrefix+mdnsHostnameOutIPv6] = "IPv6 mDNS"
	expectOut[mdnsPrefix+legacyMDNSHostnameOutIPv6] = "IPv6 legacy mDNS"
	expectOut[ssdpPrefix+ssdpUserAgentOutIPv6] = "IPv6 SSDP"
	expectIn[mdnsPrefix+mdnsHostnameInIPv6] = "IPv6 mDNS"
	expectIn[mdnsPrefix+legacyMDNSHostnameInIPv6] = "IPv6 legacy mDNS"
	expectIn[ssdpPrefix+ssdpUserAgentInIPv6] = "IPv6 SSDP"

	// Remove SSDP IPv6 expectations as we don't currently have the firewall rule.
	delete(expectOut, ssdpPrefix+ssdpUserAgentOutIPv6)
	delete(expectIn, ssdpPrefix+ssdpUserAgentInIPv6)
	// Remove inbound IPv6 expectations as the lab doesn't have IPv6 connectivity.
	delete(expectIn, mdnsPrefix+mdnsHostnameInIPv6)
	delete(expectIn, mdnsPrefix+legacyMDNSHostnameInIPv6)

	s.Log("Starting tcpdump")
	g, ctx := errgroup.WithContext(ctx)
	for _, ifname := range ifnames {
		// Start tcpdump process.
		ifname := ifname // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			tcpdump := testexec.CommandContext(ctx, "/usr/local/sbin/tcpdump", "-Ani", ifname, "-Q", "out")
			if err := streamCmd(ctx, tcpdump, expectOut); err != nil {
				return errors.Wrap(err, "outbound test failed")
			}
			return nil
		})
		g.Go(func() error {
			tcpdump := arc.BootstrapCommand(ctx, "/system/xbin/tcpdump", "-Ani", ifname, "-Q", "in")
			if err := streamCmd(ctx, tcpdump, expectIn); err != nil {
				return errors.Wrap(err, "inbound test failed")
			}
			return nil
		})
	}

	// setTexts edits multicast sender parameter by setting EditTexts' text.
	// This function shouldn't be called concurrently as it is closes over |d|.
	setTexts := func(hostname, port string) {
		if err := d.Object(ui.ID(dataID)).SetText(ctx, hostname); err != nil {
			s.Error("Failed setting hostname: ", err)
		}
		if err := d.Object(ui.ID(dataID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
			s.Error("Failed to focus on field ", dataID)
		}

		if err := d.Object(ui.ID(portID)).SetText(ctx, port); err != nil {
			s.Error("Failed setting port: ", err)
		}
		if err := d.Object(ui.ID(portID), ui.Focused(true)).WaitForExists(ctx, 30*time.Second); err != nil {
			s.Error("Failed to focus on field ", portID)
		}
	}

	toggleIPv6 := func(b bool) {
		if c, err := d.Object(ui.ID(ipv6CheckboxID)).IsChecked(ctx); err != nil {
			s.Error("Failed to get IPv6 checkbox status: ", err)
		} else if c == b {
			return
		}
		if err := d.Object(ui.ID(ipv6CheckboxID)).Click(ctx); err != nil {
			s.Error("Failed to toggle IPv6 checkbox: ", err)
		}
	}

	if ipv4Multicast {
		s.Log("Sending IPv4 multicast packets")
		toggleIPv6(false)
		// Try to send multicast packet multiple times.
		for i := 0; i < 3; i++ {
			// Send outbound multicast packets from ARC.
			// Run mDNS query.
			setTexts(mdnsHostnameOut, mdnsPort)
			if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound mDNS test: ", err)
			}
			// Run legacy mDNS query.
			setTexts(legacyMDNSHostnameOut, legacyMDNSPort)
			if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound legacy mDNS test: ", err)
			}
			// Run SSDP query
			setTexts(ssdpUserAgentOut, ssdpPort)
			if err := d.Object(ui.ID(ssdpButtonID)).Click(ctx); err != nil {
				s.Error("Failed starting outbound SSDP test: ", err)
			}

			// Set up multicast destination addresses for IPv4 multicast.
			mdnsDst := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
			ssdpDst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}

			// Send inbound multicast packets by sending multicast packet that loops back.
			for _, ifname := range ifnames {
				// Run mDNS query.
				if err := sendMDNS(mdnsHostnameIn, mdnsPort, ifname, mdnsDst); err != nil {
					s.Error("Failed starting inbound mDNS test: ", err)
				}
				// Run legacy mDNS query.
				if err := sendMDNS(legacyMDNSHostnameIn, legacyMDNSPort, ifname, mdnsDst); err != nil {
					s.Error("Failed starting inbound legacy mDNS test: ", err)
				}
				// Run SSDP query
				if err := sendSSDP(ssdpUserAgentIn, ssdpPort, ifname, ssdpDst); err != nil {
					s.Error("Failed starting inbound SSDP test: ", err)
				}
			}

		}
	}

	s.Log("Sending IPv6 multicast packets")
	toggleIPv6(true)
	// Try to send multicast packet multiple times.
	for i := 0; i < 3; i++ {
		// Send outbound multicast packets from ARC.
		// Outbound IPv6 multicast should always be tested because there is a kernel provisioned address.
		// Run IPv6 mDNS query.
		setTexts(mdnsHostnameOutIPv6, mdnsPort)
		if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
			s.Error("Failed starting outbound IPv6 mDNS test: ", err)
		}
		// Run IPv6 legacy mDNS query.
		setTexts(legacyMDNSHostnameOutIPv6, legacyMDNSPort)
		if err := d.Object(ui.ID(mdnsButtonID)).Click(ctx); err != nil {
			s.Error("Failed starting outbound IPv6 legacy mDNS test: ", err)
		}
		// Run IPv6 SSDP query
		setTexts(ssdpUserAgentOutIPv6, ssdpPort)
		if err := d.Object(ui.ID(ssdpButtonID)).Click(ctx); err != nil {
			s.Error("Failed starting outbound IPv6 SSDP test: ", err)
		}

		// Skip IPv6 inboud multicast test if there is no connectivity.
		if !ipv6Multicast {
			continue
		}

		// Set up multicast destination addresses for IPv6 multicast.
		mdnsDst := &net.UDPAddr{IP: net.ParseIP("ff02::fb"), Port: 5353}
		ssdpDst := &net.UDPAddr{IP: net.ParseIP("ff02::c"), Port: 1900}

		// Send inbound multicast packets by sending multicast packet that loops back.
		for _, ifname := range ifnames {
			// Run IPv6 mDNS query.
			if err := sendMDNS(mdnsHostnameInIPv6, mdnsPort, ifname, mdnsDst); err != nil {
				s.Error("Failed starting inbound IPv6 mDNS test: ", err)
			}
			// Run IPv6 legacy mDNS query.
			if err := sendMDNS(legacyMDNSHostnameInIPv6, legacyMDNSPort, ifname, mdnsDst); err != nil {
				s.Error("Failed starting inbound IPv6 legacy mDNS test: ", err)
			}
			// Run IPv6 SSDP query
			if err := sendSSDP(ssdpUserAgentInIPv6, ssdpPort, ifname, ssdpDst); err != nil {
				s.Error("Failed starting inbound IPv6 SSDP test: ", err)
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

// sendMDNS creates an mDNS question query for |hostname| with a socket bound to |port| and |ifname|.
// This will call sendMulticast which intentionally set a flag to loopback the multicast packet.
func sendMDNS(hostname, port, ifname string, dst *net.UDPAddr) error {
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
		return errors.Wrap(err, "failed to convert port to int")
	}
	src := &net.UDPAddr{IP: dst.IP, Port: p}

	if err := sendMulticast(b.Bytes(), src, dst, ifname); err != nil {
		return err
	}
	return nil
}

// sendSSDP creates an SSDP search query with USER-AGENT |ua| with a socket bound to |port| and |ifname|.
// This will call sendMulticast which intentionally set a flag to loopback the multicast packet.
func sendSSDP(ua, port, ifname string, dst *net.UDPAddr) error {
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
		return errors.Wrap(err, "failed to convert port to int")
	}
	src := &net.UDPAddr{IP: dst.IP, Port: p}

	if err := sendMulticast(b.Bytes(), src, dst, ifname); err != nil {
		return err
	}
	return nil
}

// sendMulticast take data |b| and send it using a temporarily create socket bound to |port| and |ifname|.
// This function intentionally set multicast loopback to true.
func sendMulticast(b []byte, src, dst *net.UDPAddr, ifname string) error {
	ifi, err := net.InterfaceByName(ifname)
	if err != nil {
		return errors.Wrap(err, "failed to get interface by name")
	}

	c, err := net.ListenMulticastUDP("udp", ifi, src)
	if err != nil {
		return errors.Wrap(err, "failed to open sending multicast socket")
	}
	defer c.Close()

	if dst.IP.To4() != nil {
		pc := ipv4.NewPacketConn(c)
		if err := pc.SetMulticastLoopback(true); err != nil {
			return errors.Wrap(err, "failed to set multicast loopback")
		}
	} else {
		pc := ipv6.NewPacketConn(c)
		if err := pc.SetMulticastLoopback(true); err != nil {
			return errors.Wrap(err, "failed to set multicast loopback")
		}
	}

	if _, err := c.WriteTo(b, dst); err != nil {
		return errors.Wrap(err, "failed to send data")
	}
	return nil
}

// streamCmd takes a command |cmd| and stream its output. It search its output for every string in map |s|.
// This function will return an error if all string in |s| is not found before context is finished.
func streamCmd(ctx context.Context, cmd *testexec.Cmd, m map[string]string) error {
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

	// Copy expectation set for checking.
	expect := m

	// Watch and wait until command have the expected outputs.
	sc := bufio.NewScanner(stdout)
	for {
		if err := ctx.Err(); err != nil {
			var e []string
			for _, v := range m {
				e = append(e, v)
			}
			return errors.Wrap(err, "failed to get "+strings.Join(e, ", "))
		}

		if !sc.Scan() {
			continue
		}

		t := sc.Text()
		for a := range expect {
			if strings.Contains(t, a) {
				delete(expect, a)
			}
		}

		if len(expect) == 0 {
			return nil
		}
	}
}
