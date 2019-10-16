// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"

	"chromiumos/tast/errors"
)

// option represents an option in a DHCP packet. Options may or may not be
// present in any given packet, depending on the configurations of the client
// and the server. Below, we'll provide different implementations of
// option to reflect that different kinds of options serialize to on the wire
// formats in different ways.
type option interface {
	pack(interface{}) ([]byte, error)
	unpack([]byte) (interface{}, error)
	name() string
	number() uint8
}

// optionBase stores the name and number fields of a given option.
type optionBase struct {
	nameField   string // human readable name for this option.
	numberField uint8  // unique identifier for this option.
}

func (o optionBase) name() string {
	return o.nameField
}

func (o optionBase) number() uint8 {
	return o.numberField
}

// appendOption serializes the option and appends the serialized bytes to the
// given byte slice.
func appendOption(buf []byte, o option, val interface{}) ([]byte, error) {
	serializedValue, err := o.pack(val)
	if err != nil {
		return nil, err
	}
	buf = append(buf, o.number(), uint8(len(serializedValue)))
	return append(buf, serializedValue...), nil
}

type byteOption struct {
	optionBase
}

func (o byteOption) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint8)
	if !ok {
		return nil, errors.New("expected uint8")
	}
	return []byte{valInt}, nil
}

func (o byteOption) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 1 {
		return nil, errors.New("expected 1 byte")
	}
	return uint8(buf[0]), nil
}

type shortOption struct {
	optionBase
}

func (o shortOption) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint16)
	if !ok {
		return nil, errors.New("expected uint16")
	}
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, valInt)
	return buf, nil
}

func (o shortOption) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 2 {
		return nil, errors.New("expected 2 bytes")
	}
	return binary.BigEndian.Uint16(buf), nil
}

type intOption struct {
	optionBase
}

func (o intOption) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint32)
	if !ok {
		return nil, errors.New("expected uint32")
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, valInt)
	return buf, nil
}

func (o intOption) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 4 {
		return nil, errors.New("expected 4 bytes")
	}
	return binary.BigEndian.Uint32(buf), nil
}

type ipAddressOption struct {
	optionBase
}

func ipToBytes(ipAddr string) ([]byte, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return nil, errors.Errorf("unable to parse IP: %q", ipAddr)
	}
	if ip.To4() == nil {
		return nil, errors.New("expected IPv4 address")
	}
	return ip.To4(), nil
}

func bytesToIP(buf []byte) (string, error) {
	byteStr := buf
	if len(buf) != 4 {
		return "", errors.New("not the expected length of an IPv4 address")
	}
	ip := net.IP(byteStr)
	return ip.String(), nil
}

func (o ipAddressOption) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	}
	return ipToBytes(valStr)
}

func (o ipAddressOption) unpack(buf []byte) (interface{}, error) {
	return bytesToIP(buf)
}

type ipListOption struct {
	optionBase
}

func (o ipListOption) pack(value interface{}) ([]byte, error) {
	valSlice, ok := value.([]string)
	if !ok {
		return nil, errors.New("expected string slice")
	}
	var buf []byte
	for _, addr := range valSlice {
		ipBytes, err := ipToBytes(addr)
		if err != nil {
			return nil, err
		}
		buf = append(buf, ipBytes...)
	}
	return buf, nil
}

func (o ipListOption) unpack(buf []byte) (interface{}, error) {
	if len(buf)%4 != 0 {
		return nil, errors.Errorf("%d is not a multiple of 4", len(buf))
	}
	var ipList []string
	for len(buf) >= 4 {
		ipString, err := bytesToIP(buf[:4])
		if err != nil {
			return nil, err
		}
		ipList = append(ipList, ipString)
		buf = buf[4:]
	}
	return ipList, nil
}

type rawOption struct {
	optionBase
}

func (o rawOption) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.([]byte)
	if !ok {
		return nil, errors.New("expected byte slice")
	}
	return valStr, nil
}

func (o rawOption) unpack(buf []byte) (interface{}, error) {
	return buf, nil
}

// classlessStaticRoutesOption is a RFC 3442 compliant classless static route
// option parser and serializer. The symbolic "value" packed and unpacked from
// this class is a slice of staticRoutes (defined below).
type classlessStaticRoutesOption struct {
	optionBase
}

type staticRoute struct {
	prefixSize uint8
	destAddr   string
	routerAddr string
}

func (o classlessStaticRoutesOption) pack(value interface{}) ([]byte, error) {
	routeList, ok := value.([]staticRoute)
	if !ok {
		return nil, errors.New("expected staticRoute slice")
	}
	var buf []byte
	for _, route := range routeList {
		buf = append(buf, route.prefixSize)
		destAddrCount := (route.prefixSize + 7) / 8
		destAddrBytes, err := ipToBytes(route.destAddr)
		if err != nil {
			return nil, err
		}
		buf = append(buf, destAddrBytes[:destAddrCount]...)
		routerAddrBytes, err := ipToBytes(route.routerAddr)
		if err != nil {
			return nil, err
		}
		buf = append(buf, routerAddrBytes...)
	}
	return buf, nil
}

func (o classlessStaticRoutesOption) unpack(buf []byte) (interface{}, error) {
	var routeList []staticRoute
	for len(buf) > 0 {
		prefixSize := int(buf[0])
		buf = buf[1:]
		destAddrCount := (prefixSize + 7) / 8
		if destAddrCount+4 > len(buf) {
			return nil, errors.New("classless domain list is corrupted")
		}
		destAddrBytes := make([]byte, 4)
		copy(destAddrBytes, buf[:destAddrCount])
		destAddr, err := bytesToIP(destAddrBytes)
		buf = buf[destAddrCount:]
		if err != nil {
			return nil, err
		}
		routerAddrBytes := make([]byte, 4)
		copy(routerAddrBytes, buf[:4])
		routerAddr, err := bytesToIP(routerAddrBytes)
		buf = buf[4:]
		if err != nil {
			return nil, err
		}
		routeList = append(routeList, staticRoute{uint8(prefixSize), destAddr, routerAddr})
	}
	return routeList, nil
}

// domainListOption is a RFC 1035 compliant domain list option parser and
// serializer.
// There are some clever compression optimizations that it does not implement
// for serialization, but correctly parses.  This should be sufficient for
// testing.
type domainListOption struct {
	optionBase
}

func (o domainListOption) pack(value interface{}) ([]byte, error) {
	domainList, ok := value.([]string)
	if !ok {
		return nil, errors.New("expected string slice")
	}
	var buf []byte
	for _, domain := range domainList {
		for _, part := range strings.Split(domain, ".") {
			if len(part) >= 256 {
				return nil, errors.Errorf("len(part) = %d, expected length less than 256", len(part))
			}
			buf = append(buf, uint8(len(part)))
			buf = append(buf, part...)
		}
		buf = append(buf, uint8(0))
	}
	return buf, nil
}

func (o domainListOption) unpack(buf []byte) (interface{}, error) {
	var domainList []string
	offset := 0
	for offset < len(buf) {
		newOffset, domainParts, err := readDomainName(buf, offset)
		if err != nil {
			return nil, err
		}
		domainName := strings.Join(domainParts, ".")
		domainList = append(domainList, domainName)
		if newOffset <= offset {
			return nil, errors.New("parsing logic error is letting domain list parsing go on forever")
		}
		offset = newOffset
	}
	return domainList, nil
}

// Various RFC's let you finish a domain name by pointing to an existing domain
// name rather than repeating the same suffix.  All such pointers are two buf
// long, specify the offset in the byte string, and begin with |pointerPrefix|
// to distinguish them from normal characters.
const pointerPrefix = '\xC0'

// readDomainName recursively parses a domain name from a domain name list.
func readDomainName(buf []byte, offset int) (int, []string, error) {
	var parts []string
	for {
		if offset >= len(buf) {
			return 0, nil, errors.New("domain list ended without a NULL byte")
		}
		maybePartLen := int(buf[offset])
		offset++
		if maybePartLen == 0 {
			return offset, parts, nil
		} else if (maybePartLen & pointerPrefix) == pointerPrefix {
			if offset >= len(buf) {
				return 0, nil, errors.New("missing second byte of domain suffix pointer")
			}
			maybePartLen &= ^pointerPrefix
			pointerOffset := ((maybePartLen << 8) + int(buf[offset]))
			offset++
			_, moreParts, err := readDomainName(buf, pointerOffset)
			if err != nil {
				return 0, nil, err
			}
			parts = append(parts, moreParts...)
			return offset, parts, nil
		} else {
			partLen := maybePartLen
			if offset+partLen >= len(buf) {
				return 0, nil, errors.New("part of a domain goes beyond data length")
			}
			parts = append(parts, string(buf[offset:offset+partLen]))
			offset += partLen
		}
	}
}

// field represents a required field in a DHCP packet. Similar to
// option, we'll implement this interface to reflect that different
// fields serialize toon the wire formats in different ways.
type field interface {
	pack(interface{}) ([]byte, error)
	unpack([]byte) (interface{}, error)
	name() string
	offset() int
	size() int
}

type fieldBase struct {
	nameField   string // human readable name for this field.
	offsetField int    // defines the starting byte of the field in the binary packet string.
	sizeField   int    // defines the fixed size that must be respected
}

func appendField(buf []byte, f field, val interface{}) ([]byte, error) {
	buf = append(buf, make([]byte, f.offset()-len(buf))...)
	serializedValue, err := f.pack(val)
	if err != nil {
		return nil, err
	}
	return append(buf, serializedValue...), nil
}

func (f fieldBase) name() string {
	return f.nameField
}

func (f fieldBase) offset() int {
	return f.offsetField
}

func (f fieldBase) size() int {
	return f.sizeField
}

type byteField struct {
	fieldBase
}

func (f byteField) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint8)
	if !ok {
		return nil, errors.New("expected uint8")
	}
	return []byte{valInt}, nil
}

func (f byteField) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 1 {
		return nil, errors.New("expected 1 byte")
	}
	return uint8(buf[0]), nil
}

type shortField struct {
	fieldBase
}

func (f shortField) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint16)
	if !ok {
		return nil, errors.New("expected uint16")
	}
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, valInt)
	return buf, nil
}

func (f shortField) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 2 {
		return nil, errors.New("expected 2 bytes")
	}
	return binary.BigEndian.Uint16(buf), nil
}

type intField struct {
	fieldBase
}

func (f intField) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint32)
	if !ok {
		return nil, errors.New("expected uint32")
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, valInt)
	return buf, nil
}

func (f intField) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 4 {
		return nil, errors.New("expected 4 bytes")
	}
	return binary.BigEndian.Uint32(buf), nil
}

type hwAddrField struct {
	fieldBase
}

func (f hwAddrField) pack(value interface{}) ([]byte, error) {
	valBytes, ok := value.([]byte)
	if !ok {
		return nil, errors.New("expected byte slice")
	} else if len(valBytes) > 16 {
		return nil, errors.New("expected byte slice of length no more than 16")
	}
	return valBytes, nil
}

func (f hwAddrField) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 16 {
		return nil, errors.New("expected byte slice of length 16")
	}
	return buf, nil
}

type serverNameField struct {
	fieldBase
}

func (f serverNameField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	} else if len(valStr) > 64 {
		return nil, errors.New("expected string of length no more than 64")
	}
	buf := make([]byte, 64)
	copy(buf, valStr)
	return buf, nil
}

func (f serverNameField) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 64 {
		return nil, errors.New("expected byte slice of length 64")
	}
	return strings.Trim(string(buf), "\x00"), nil
}

type bootFileField struct {
	fieldBase
}

func (f bootFileField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	} else if len(valStr) > 128 {
		return nil, errors.New("expected string of length no more than 128")
	}
	buf := make([]byte, 128)
	copy(buf, valStr)
	return buf, nil
}

func (f bootFileField) unpack(buf []byte) (interface{}, error) {
	if len(buf) != 128 {
		return nil, errors.New("expected byte slice of length 128")
	}
	return strings.Trim(string(buf), "\x00"), nil
}

type ipAddressField struct {
	fieldBase
}

func (f ipAddressField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	}
	return ipToBytes(valStr)
}

func (f ipAddressField) unpack(buf []byte) (interface{}, error) {
	return bytesToIP(buf)
}

// DHCP fields.
var (
	// These are required in every DHCP packet. Without these fields, the packet
	// will not even pass dhcpPacket.isValid
	op             = byteField{fieldBase{"op", 0, 1}}
	hwType         = byteField{fieldBase{"htype", 1, 1}}
	hwAddrLen      = byteField{fieldBase{"hlen", 2, 1}}
	relayHops      = byteField{fieldBase{"hops", 3, 1}}
	transactionID  = intField{fieldBase{"xid", 4, 4}}
	timeSinceStart = shortField{fieldBase{"secs", 8, 2}}
	flags          = shortField{fieldBase{"flags", 10, 2}}
	clientIP       = ipAddressField{fieldBase{"ciaddr", 12, 4}}
	yourIP         = ipAddressField{fieldBase{"yiaddr", 16, 4}}
	serverIP       = ipAddressField{fieldBase{"siaddr", 20, 4}}
	gatewayIP      = ipAddressField{fieldBase{"giaddr", 24, 4}}
	clientHWAddr   = hwAddrField{fieldBase{"chaddr", 28, 16}}

	// The following two fields are considered "legacy BOOTP" fields but may
	// sometimes be used by DHCP clients.
	legacyServerName = serverNameField{fieldBase{"servername", 44, 64}}
	legacyBootFile   = bootFileField{fieldBase{"bootfile", 108, 128}}
	magicCookie      = intField{fieldBase{"magic_cookie", 236, 4}}
)

// DHCP options.
var (
	timeOffset                = intOption{optionBase{"time_offset", 2}}
	routers                   = ipListOption{optionBase{"routers", 3}}
	subnetMask                = ipAddressOption{optionBase{"subnet_mask", 1}}
	timeServers               = ipListOption{optionBase{"time_servers", 4}}
	nameServers               = ipListOption{optionBase{"name_servers", 5}}
	dnsServers                = ipListOption{optionBase{"dns_servers", 6}}
	logServers                = ipListOption{optionBase{"log_servers", 7}}
	cookieServers             = ipListOption{optionBase{"cookie_servers", 8}}
	lprServers                = ipListOption{optionBase{"lpr_servers", 9}}
	impressServers            = ipListOption{optionBase{"impress_servers", 10}}
	resourceLOCServers        = ipListOption{optionBase{"resource_loc_servers", 11}}
	hostName                  = rawOption{optionBase{"host_name", 12}}
	bootFileSize              = shortOption{optionBase{"boot_file_size", 13}}
	meritDumpFile             = rawOption{optionBase{"merit_dump_file", 14}}
	domainName                = rawOption{optionBase{"domain_name", 15}}
	swapServer                = ipAddressOption{optionBase{"swap_server", 16}}
	rootPath                  = rawOption{optionBase{"root_path", 17}}
	extensions                = rawOption{optionBase{"extensions", 18}}
	interfaceMTU              = shortOption{optionBase{"interface_mtu", 26}}
	vendorEncapsulatedOptions = rawOption{optionBase{"vendor_encapsulated_options", 43}}
	requestedIP               = ipAddressOption{optionBase{"requested_ip", 50}}
	ipLeaseTime               = intOption{optionBase{"ip_lease_time", 51}}
	optionOverload            = byteOption{optionBase{"option_overload", 52}}
	dhcpMessageType           = byteOption{optionBase{"dhcp_message_type", 53}}
	serverID                  = ipAddressOption{optionBase{"server_id", 54}}
	parameterRequestList      = rawOption{optionBase{"parameter_request_list", 55}}
	message                   = rawOption{optionBase{"message", 56}}
	maxDHCPMessageSize        = shortOption{optionBase{"max_dhcp_message_size", 57}}
	renewalT1TimeValue        = intOption{optionBase{"renewal_t1_time_value", 58}}
	rebindingT2TimeValue      = intOption{optionBase{"rebinding_t2_time_value", 59}}
	vendorID                  = rawOption{optionBase{"vendor_id", 60}}
	clientID                  = rawOption{optionBase{"client_id", 61}}
	tftpServerName            = rawOption{optionBase{"tftp_server_name", 66}}
	bootfileName              = rawOption{optionBase{"bootfile_name", 67}}
	fullyQualifiedDomainName  = rawOption{optionBase{"fqdn", 81}}
	dnsDomainSearchList       = domainListOption{optionBase{"domain_search_list", 119}}
	classlessStaticRoutes     = classlessStaticRoutesOption{optionBase{"classless_static_routes", 121}}
	webProxyAutoDiscovery     = rawOption{optionBase{"wpad", 252}}
)

type msgType struct {
	name      string
	optionVal uint8
}

// From RFC2132, the valid DHCP message types are as follows.
var (
	unknown   = msgType{"UNKNOWN", 0}
	discovery = msgType{"DISCOVERY", 1}
	offer     = msgType{"OFFER", 2}
	request   = msgType{"REQUEST", 3}
	decline   = msgType{"DECLINE", 4}
	ack       = msgType{"ACK", 5}
	nak       = msgType{"NAK", 6}
	release   = msgType{"RELEASE", 7}
	inform    = msgType{"INFORM", 8}
)

const (
	// This is per RFC 2131.  The wording doesn't seem to say that the packets
	// must be this big, but that has been the historic assumption in
	// implementations.
	minPacketSize = 300
	ipv4Null      = "0.0.0.0"
)

// Option constants.
const (
	// Unlike every other option the pad and end options are just single bytes
	// "\x00" and "\xff" (without length or data fields).
	optionPad          = 0
	optionEnd          = 255
	optionsStartOffset = 240
)

// Field values.
const (
	// The op field in an IPv4 packet is either 1 or 2 depending on whether the
	// packet is from a server or from a client.
	opClientRequest  = uint8(1)
	opServerResponse = uint8(2)

	// 1 == 10mb ethernet hardware address type (aka MAC).
	hwType10MBEth = uint8(1)

	// MAC addresses are still 6 bytes long.
	hwAddrLen10MBEth = uint8(6)
	magicCookieVal   = uint32(0x63825363)
)

var (
	commonFields = []field{
		op,
		hwType,
		hwAddrLen,
		relayHops,
		transactionID,
		timeSinceStart,
		flags,
		clientIP,
		yourIP,
		serverIP,
		gatewayIP,
		clientHWAddr,
	}

	requiredFields = append(append([]field(nil), commonFields...), magicCookie)

	allFields = append(append([]field(nil), commonFields...), []field{legacyServerName, legacyBootFile, magicCookie}...)

	// allOptions are possible options that may not be in every packet.
	// Frequently, the client can include a bunch of options that indicate that it
	// would like to receive information about time servers, routers, lpr servers,
	// and much more, but the DHCP server can usually ignore those requests.
	//
	// Eventually, each option is encoded as:
	//     <option.number(), option.size(), [slice of option.size() bytes]>
	// Unlike fields, which make up a fixed packet format, options can be in any
	// order, except where they cannot.  For instance, option 1 must follow option
	// 3 if both are supplied.  For this reason, potential options are in this
	// list, and added to the packet in this order every time.
	//
	// size < 0 indicates that this is variable length field of at least
	// abs(length) bytes in size.
	allOptions = []option{
		timeOffset,
		routers,
		subnetMask,
		timeServers,
		nameServers,
		dnsServers,
		logServers,
		cookieServers,
		lprServers,
		impressServers,
		resourceLOCServers,
		hostName,
		bootFileSize,
		meritDumpFile,
		swapServer,
		domainName,
		rootPath,
		extensions,
		interfaceMTU,
		vendorEncapsulatedOptions,
		requestedIP,
		ipLeaseTime,
		optionOverload,
		dhcpMessageType,
		serverID,
		parameterRequestList,
		message,
		maxDHCPMessageSize,
		renewalT1TimeValue,
		rebindingT2TimeValue,
		vendorID,
		clientID,
		tftpServerName,
		bootfileName,
		fullyQualifiedDomainName,
		dnsDomainSearchList,
		classlessStaticRoutes,
		webProxyAutoDiscovery,
	}

	msgTypeByNum = []msgType{
		unknown,
		discovery,
		offer,
		request,
		decline,
		ack,
		nak,
		release,
		inform,
	}

	defaultParameterRequestList = []uint8{
		requestedIP.number(),
		ipLeaseTime.number(),
		serverID.number(),
		subnetMask.number(),
		routers.number(),
		dnsServers.number(),
		hostName.number(),
	}
)

func getDHCPOptionByNumber(number uint8) option {
	for _, option := range allOptions {
		if option.number() == number {
			return option
		}
	}
	return nil
}

type optionMap map[option]interface{}
type fieldMap map[field]interface{}

// dhcpPacket is a class that represents a single DHCP packet and contains some
// logic to create and parse binary strings containing on the wire DHCP packets.
// While you could call |newDHCPPacket| explicitly, most users should use the
// factories to construct packets with reasonable default values in most of
// the fields, even if those values are zeros.
type dhcpPacket struct {
	options optionMap
	fields  fieldMap
}

// createDiscovery creates a discovery packet.
// Fill in fields of a DHCP packet as if it were being sent from |macAddr|.
// Requests subnet masks, broadcast addresses, router addresses, DNS addresses,
// domain search lists, client host name, and NTP server addresses. Note that
// the offer packet received in response to this packet will probably not
// contain all of that information.
func createDiscovery(macAddr []byte) (*dhcpPacket, error) {
	// MAC addresses are actually only 6 bytes long, however, for whatever reason,
	// DHCP allocated 12 bytes to this field.  Ease the burden on developers and
	// hide this detail.
	macAddr = append(append([]byte{}, macAddr...), bytes.Repeat([]byte{optionPad}, 12-len(macAddr))...)
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(op, opClientRequest)
	packet.setField(hwType, hwType10MBEth)
	packet.setField(hwAddrLen, hwAddrLen10MBEth)
	packet.setField(relayHops, uint8(0))
	packet.setField(transactionID, rand.Uint32())
	packet.setField(timeSinceStart, uint16(0))
	packet.setField(flags, uint16(0))
	packet.setField(clientIP, ipv4Null)
	packet.setField(yourIP, ipv4Null)
	packet.setField(serverIP, ipv4Null)
	packet.setField(gatewayIP, ipv4Null)
	packet.setField(clientHWAddr, macAddr)
	packet.setField(magicCookie, magicCookieVal)
	packet.setOption(dhcpMessageType, discovery.optionVal)
	return packet, nil
}

// createOffer creates an offer packet, given some fields that tie the
// packet to a particular offer.
func createOffer(txnID uint32, macAddr []byte, offerIP string, svrIP string) (*dhcpPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(op, opServerResponse)
	packet.setField(hwType, hwType10MBEth)
	packet.setField(hwAddrLen, hwAddrLen10MBEth)
	packet.setField(relayHops, uint8(0))
	packet.setField(transactionID, txnID)
	packet.setField(timeSinceStart, uint16(0))
	packet.setField(flags, uint16(0))
	packet.setField(clientIP, ipv4Null)
	packet.setField(yourIP, offerIP)
	packet.setField(serverIP, svrIP)
	packet.setField(gatewayIP, ipv4Null)
	packet.setField(clientHWAddr, macAddr)
	packet.setField(magicCookie, magicCookieVal)
	packet.setOption(dhcpMessageType, offer.optionVal)
	return packet, nil
}

func createRequest(txnID uint32, macAddr []byte) (*dhcpPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(op, opClientRequest)
	packet.setField(hwType, hwType10MBEth)
	packet.setField(hwAddrLen, hwAddrLen10MBEth)
	packet.setField(relayHops, uint8(0))
	packet.setField(transactionID, txnID)
	packet.setField(timeSinceStart, uint16(0))
	packet.setField(flags, uint16(0))
	packet.setField(clientIP, ipv4Null)
	packet.setField(yourIP, ipv4Null)
	packet.setField(serverIP, ipv4Null)
	packet.setField(gatewayIP, ipv4Null)
	packet.setField(clientHWAddr, macAddr)
	packet.setField(magicCookie, magicCookieVal)
	packet.setOption(dhcpMessageType, request.optionVal)
	return packet, nil
}

func createAck(txnID uint32, macAddr []byte, grantedIP string, svrIP string) (*dhcpPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(op, opServerResponse)
	packet.setField(hwType, hwType10MBEth)
	packet.setField(hwAddrLen, hwAddrLen10MBEth)
	packet.setField(relayHops, uint8(0))
	packet.setField(transactionID, txnID)
	packet.setField(timeSinceStart, uint16(0))
	packet.setField(flags, uint16(0))
	packet.setField(clientIP, ipv4Null)
	packet.setField(yourIP, grantedIP)
	packet.setField(serverIP, svrIP)
	packet.setField(gatewayIP, ipv4Null)
	packet.setField(clientHWAddr, macAddr)
	packet.setField(magicCookie, magicCookieVal)
	packet.setOption(dhcpMessageType, ack.optionVal)
	return packet, nil
}

func createNAK(txnID uint32, macAddr []byte) (*dhcpPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(op, opServerResponse)
	packet.setField(hwType, hwType10MBEth)
	packet.setField(hwAddrLen, hwAddrLen10MBEth)
	packet.setField(relayHops, uint8(0))
	packet.setField(transactionID, txnID)
	packet.setField(timeSinceStart, uint16(0))
	packet.setField(flags, uint16(0))
	packet.setField(clientIP, ipv4Null)
	packet.setField(yourIP, ipv4Null)
	packet.setField(serverIP, ipv4Null)
	packet.setField(gatewayIP, ipv4Null)
	packet.setField(clientHWAddr, macAddr)
	packet.setField(magicCookie, magicCookieVal)
	packet.setOption(dhcpMessageType, nak.optionVal)
	return packet, nil
}

// newDHCPPacket creates a dhcpPacket, filling in fields from a byte string if
// given.
// Assumes that the packet starts at offset 0 in the binary string. This
// includes the fields and options. Fields are different from options in that we
// bother to decode these into more usable data types like integers rather than
// keeping them as raw byte strings. Fields are also required to exist, unlike
// options which may not.
// Each option is encoded as a tuple <option number, length, data> where option
// number is a byte indicating the type of option, length indicates the number
// of bytes in the data for option, and data is a length array of bytes. The
// only exceptions to this rule are the 0 and 255 options, which have 0 data
// length, and no length byte. These tuples are then simply appended to each
// other. This encoding is the same as the BOOTP vendor extension field
// encoding.
func newDHCPPacket(buf []byte) (*dhcpPacket, error) {
	var packet dhcpPacket
	packet.options = make(optionMap)
	packet.fields = make(fieldMap)
	if len(buf) == 0 {
		return &packet, nil
	}
	if len(buf) < optionsStartOffset+1 {
		return nil, errors.New("invalid byte string for packet")
	}
	for _, field := range allFields {
		fieldVal, err := field.unpack(buf[field.offset() : field.offset()+field.size()])
		if err != nil {
			return nil, err
		}
		packet.fields[field] = fieldVal
	}
	offset := optionsStartOffset
	var domainSearchListByteString []byte
	for offset < len(buf) && buf[offset] != optionEnd {
		dataType := buf[offset]
		offset++
		if dataType == optionPad {
			continue
		}
		dataLength := int(buf[offset])
		offset++
		data := buf[offset : offset+dataLength]
		offset += dataLength
		option := getDHCPOptionByNumber(dataType)
		if option == nil {
			continue
		}
		if option == dnsDomainSearchList {
			// In a cruel twist of fate, the server is allowed to give multiple
			// options with this number. The client is expected to concatenate the
			// byte strings together and use it as a single value.
			domainSearchListByteString = append(domainSearchListByteString, data...)
			continue
		}
		optionVal, err := (option).unpack(data)
		if err != nil {
			return nil, err
		}
		packet.options[option] = optionVal
	}
	if len(domainSearchListByteString) > 0 {
		domainSearchListVal, err := dnsDomainSearchList.unpack(domainSearchListByteString)
		if err != nil {
			return nil, err
		}
		packet.options[dnsDomainSearchList] = domainSearchListVal
	}
	return &packet, nil
}

func (d *dhcpPacket) clientHWAddr() ([]byte, error) {
	addr, ok := d.fields[clientHWAddr]
	if !ok {
		return nil, errors.New("client addr field not found")
	}
	addrBytes, ok := addr.([]byte)
	if !ok {
		return nil, errors.New("expected byte slice type")
	}
	return addrBytes, nil
}

// isValid checks that we have (at a minimum) values for all the required
// fields, and that the magic cookie is set correctly.
func (d *dhcpPacket) isValid() error {
	for _, field := range requiredFields {
		if d.fields[field] == nil {
			return errors.Errorf("required field %s is missing from DHCP packet", field.name())
		}
	}
	if d.fields[magicCookie] != magicCookieVal {
		return errors.Errorf("magic cookie value is %x, expected %x", d.fields[magicCookie], magicCookieVal)
	}
	return nil
}

// msgType gets the value of the DHCP Message Type option in this packet.
// If the option is not present, or the value of the option is not recognized,
// returns msgTypeUnknown.
func (d *dhcpPacket) msgType() (msgType, error) {
	typeNum, ok := d.options[dhcpMessageType]
	if !ok {
		return unknown, errors.New("message type option not found")
	}
	typeNumInt, ok := typeNum.(uint8)
	if !ok {
		return unknown, errors.New("expected uint8 type")
	}
	if typeNumInt > 0 && int(typeNumInt) < len(msgTypeByNum) {
		return msgTypeByNum[typeNumInt], nil
	}
	return unknown, errors.New("invalid message type")
}

func (d *dhcpPacket) txnID() (uint32, error) {
	ID, ok := d.fields[transactionID]
	if !ok {
		return 0, errors.New("transaction ID field not found")
	}
	IDInt, ok := ID.(uint32)
	if !ok {
		return 0, errors.New("expected uint32 type")
	}
	return IDInt, nil
}

func (d *dhcpPacket) field(field field) interface{} {
	return d.fields[field]
}

func (d *dhcpPacket) option(option option) interface{} {
	return d.options[option]
}

func (d *dhcpPacket) setField(field field, fieldValue interface{}) {
	d.fields[field] = fieldValue
}

func (d *dhcpPacket) setOption(option option, optionValue interface{}) {
	d.options[option] = optionValue
}

func (d *dhcpPacket) marshal() ([]byte, error) {
	if err := d.isValid(); err != nil {
		return nil, errors.Wrap(err, "invalid packet")
	}
	var data []byte
	var err error
	for _, field := range allFields {
		fieldVal, ok := d.fields[field]
		if !ok {
			continue
		}
		data, err = appendField(data, field, fieldVal)
		if err != nil {
			return nil, err
		}
	}
	for _, option := range allOptions {
		optionVal, ok := d.options[option]
		if !ok {
			continue
		}
		data, err = appendOption(data, option, optionVal)
		if err != nil {
			return nil, err
		}
	}
	data = append(data, optionEnd)
	return append(data, bytes.Repeat([]byte{optionPad}, minPacketSize-len(data))...), nil
}

func (d *dhcpPacket) String() string {
	var options, fields []string
	for field, fieldVal := range d.fields {
		fieldStr := fmt.Sprintf("%v=%v", field.name(), fieldVal)
		fields = append(fields, fieldStr)
	}
	for option, optionVal := range d.options {
		optionStr := fmt.Sprintf("%v=%v", option.name(), optionVal)
		options = append(options, optionStr)
	}
	return fmt.Sprintf("<DHCPPacket fields=[%s], options=[%s]>", strings.Join(fields, ","), strings.Join(options, ","))
}
