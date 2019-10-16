// Copyright 2019 The Chromium OS Authors. All rights reserved.
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

// optionInterface represents an option in a DHCP packet. Options may or may not
// be present in any given packet, depending on the configurations of the client
// and the server. Below, we'll provide different implementations of
// optionInterface to reflect that different kinds of options serialize to on
// the wire formats in different ways.
type optionInterface interface {
	pack(interface{}) ([]byte, error)
	unpack([]byte) (interface{}, error)
	name() string
	number() uint8
}

// option stores the name and number fields of a given option.
type option struct {
	nameField   string // human readable name for this option.
	numberField uint8  // unique identifier for this option.
}

func (o option) name() string {
	return o.nameField
}

func (o option) number() uint8 {
	return o.numberField
}

// appendOption serializes the option and appends the serialized bytes to the
// given byte slice.
func appendOption(buf []byte, o optionInterface, val interface{}) ([]byte, error) {
	serializedValue, err := o.pack(val)
	if err != nil {
		return nil, err
	}
	buf = append(buf, o.number(), uint8(len(serializedValue)))
	return append(buf, serializedValue...), nil
}

type byteOption struct {
	option
}

func (o byteOption) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint8)
	if !ok {
		return nil, errors.New("expected uint8")
	}
	return []byte{valInt}, nil
}

func (o byteOption) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 1 {
		return nil, errors.New("expected 1 byte")
	}
	return uint8(bytes[0]), nil
}

type shortOption struct {
	option
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

func (o shortOption) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 2 {
		return nil, errors.New("expected 2 bytes")
	}
	return binary.BigEndian.Uint16(bytes), nil
}

type intOption struct {
	option
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

func (o intOption) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 4 {
		return nil, errors.New("expected 4 bytes")
	}
	return binary.BigEndian.Uint32(bytes), nil
}

type ipAddressOption struct {
	option
}

func ipToBytes(IPAddr string) ([]byte, error) {
	IP := net.ParseIP(IPAddr)
	if IP == nil {
		return nil, errors.Errorf("unable to parse IP: %s", IPAddr)
	}
	return IP.To4(), nil
}

func bytesToIP(buf []byte) (string, error) {
	byteStr := string(buf)
	if len(buf) > 4 {
		return "", errors.New("byte string is too long")
	}
	byteStr += strings.Repeat("\x00", 4-len(buf))
	IP := net.IP(byteStr)
	return IP.String(), nil
}

func (o ipAddressOption) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	}
	return ipToBytes(valStr)
}

func (o ipAddressOption) unpack(bytes []byte) (interface{}, error) {
	return bytesToIP(bytes)
}

type ipListOption struct {
	option
}

func (o ipListOption) pack(value interface{}) ([]byte, error) {
	valSlice, ok := value.([]string)
	if !ok {
		return nil, errors.New("expected string slice")
	}
	var bytes []byte
	for _, addr := range valSlice {
		IPBytes, err := ipToBytes(addr)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, IPBytes...)
	}
	return bytes, nil
}

func (o ipListOption) unpack(bytes []byte) (interface{}, error) {
	if len(bytes)%4 != 0 {
		return nil, errors.New("unable to parse list")
	}
	var IPList []string
	for i := 0; i < len(bytes); i += 4 {
		IPString, err := bytesToIP(bytes[i : i+4])
		if err != nil {
			return nil, err
		}
		IPList = append(IPList, IPString)
	}
	return IPList, nil
}

type rawOption struct {
	option
}

func (o rawOption) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.([]byte)
	if !ok {
		return nil, errors.New("expected byte slice")
	}
	return valStr, nil
}

func (o rawOption) unpack(bytes []byte) (interface{}, error) {
	return bytes, nil
}

type byteListOption struct {
	option
}

func (o byteListOption) pack(value interface{}) ([]byte, error) {
	valBytes, ok := value.([]byte)
	if !ok {
		return nil, errors.New("expected byte slice")
	}
	return valBytes, nil
}

func (o byteListOption) unpack(bytes []byte) (interface{}, error) {
	return bytes, nil
}

// classlessStaticRoutesOption is a RFC 3442 compliant classless static route
// option parser and serializer. The symbolic "value" packed and unpacked from
// this class is a slice of staticRoutes (defined below).
type classlessStaticRoutesOption struct {
	option
}

type staticRoute struct {
	prefixSize         uint8
	destinationAddress string
	routerAddress      string
}

func (o classlessStaticRoutesOption) pack(value interface{}) ([]byte, error) {
	routeList, ok := value.([]staticRoute)
	if !ok {
		return nil, errors.New("expected staticRoute slice")
	}
	var byteStr string
	for _, route := range routeList {
		byteStr += string([]byte{route.prefixSize})
		destinationAddressCount := (route.prefixSize + 7) / 8
		destinationAddressBytes, err := ipToBytes(route.destinationAddress)
		if err != nil {
			return nil, err
		}
		byteStr += string(destinationAddressBytes)[:destinationAddressCount]
		routerAddressBytes, err := ipToBytes(route.routerAddress)
		if err != nil {
			return nil, err
		}
		byteStr += string(routerAddressBytes)
	}
	return []byte(byteStr), nil
}

func (o classlessStaticRoutesOption) unpack(bytes []byte) (interface{}, error) {
	var routeList []staticRoute
	offset := 0
	for offset < len(bytes) {
		prefixSize := int(bytes[offset])
		destinationAddressCount := (prefixSize + 7) / 8
		entryEnd := offset + 1 + destinationAddressCount + 4
		if entryEnd > len(bytes) {
			return nil, errors.New("classless domain list is corrupted")
		}
		offset++
		destinationAddressEnd := offset + destinationAddressCount
		destinationAddress, err := bytesToIP(bytes[offset:destinationAddressEnd])
		if err != nil {
			return nil, err
		}
		routerAddress, err := bytesToIP(bytes[destinationAddressEnd:entryEnd])
		if err != nil {
			return nil, err
		}
		routeList = append(routeList, staticRoute{uint8(prefixSize), destinationAddress, routerAddress})
		offset = entryEnd
	}
	return routeList, nil
}

// domainListOption is a RFC 1035 compliant domain list option parser and
// serializer.
// There are some clever compression optimizations that it does not implement
// for serialization, but correctly parses.  This should be sufficient for
// testing.
type domainListOption struct {
	option
}

func (o domainListOption) pack(value interface{}) ([]byte, error) {
	domainList, ok := value.([]string)
	if !ok {
		return nil, errors.New("expected string slice")
	}
	byteStr := ""
	for _, domain := range domainList {
		for _, part := range strings.Split(domain, ".") {
			byteStr += string([]byte{uint8(len(part))})
			byteStr += part
		}
		byteStr += "\x00"
	}
	return []byte(byteStr), nil
}

func (o domainListOption) unpack(bytes []byte) (interface{}, error) {
	var domainList []string
	offset := 0
	for offset < len(bytes) {
		newOffset, domainParts, err := o.readDomainName(bytes, offset)
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
// name rather than repeating the same suffix.  All such pointers are two bytes
// long, specify the offset in the byte string, and begin with |pointerPrefix|
// to distinguish them from normal characters.
const pointerPrefix = '\xC0'

// readDomainName recursively parses a domain name from a domain name list.
func (o domainListOption) readDomainName(bytes []byte, offset int) (int, []string, error) {
	var parts []string
	for {
		if offset >= len(bytes) {
			return 0, nil, errors.New("domain list ended without a NULL byte")
		}
		maybePartLen := int(bytes[offset])
		offset++
		if maybePartLen == 0 {
			return offset, parts, nil
		} else if (maybePartLen & pointerPrefix) == pointerPrefix {
			if offset >= len(bytes) {
				return 0, nil, errors.New("missing second byte of domain suffix pointer")
			}
			maybePartLen &= ^pointerPrefix
			pointerOffset := ((maybePartLen << 8) + int(bytes[offset]))
			offset++
			_, moreParts, err := o.readDomainName(bytes, pointerOffset)
			if err != nil {
				return 0, nil, err
			}
			parts = append(parts, moreParts...)
			return offset, parts, nil
		} else {
			partLen := maybePartLen
			if offset+partLen >= len(bytes) {
				return 0, nil, errors.New("part of a domain goes beyond data length")
			}
			parts = append(parts, string(bytes[offset:offset+partLen]))
			offset += partLen
		}
	}
}

// fieldInterface represents a required field in a DHCP packet. Similar to
// optionInterface, we'll implement this interface to reflect that different
// fields serialize toon the wire formats in different ways.
type fieldInterface interface {
	pack(interface{}) ([]byte, error)
	unpack([]byte) (interface{}, error)
	name() string
	offset() int
	size() int
}

type field struct {
	nameField   string // human readable name for this field.
	offsetField int    // defines the starting byte of the field in the binary packet string.
	sizeField   int    // defines the fixed size that must be respected
}

func appendField(buf []byte, f fieldInterface, val interface{}) ([]byte, error) {
	buf = append(buf, []byte(strings.Repeat("\x00", f.offset()-len(buf)))...)
	serializedValue, err := f.pack(val)
	if err != nil {
		return nil, err
	}
	return append(buf, serializedValue...), nil
}

func (f field) name() string {
	return f.nameField
}

func (f field) offset() int {
	return f.offsetField
}

func (f field) size() int {
	return f.sizeField
}

type byteField struct {
	field
}

func (f byteField) pack(value interface{}) ([]byte, error) {
	valInt, ok := value.(uint8)
	if !ok {
		return nil, errors.New("expected uint8")
	}
	return []byte{valInt}, nil
}

func (f byteField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 1 {
		return nil, errors.New("expected 1 byte")
	}
	return uint8(bytes[0]), nil
}

type shortField struct {
	field
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

func (f shortField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 2 {
		return nil, errors.New("expected 2 bytes")
	}
	return binary.BigEndian.Uint16(bytes), nil
}

type intField struct {
	field
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

func (f intField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 4 {
		return nil, errors.New("expected 4 bytes")
	}
	return binary.BigEndian.Uint32(bytes), nil
}

type hwAddrField struct {
	field
}

func (f hwAddrField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	} else if len(valStr) > 16 {
		return nil, errors.New("expected string of length no more than 16")
	}
	valStr += strings.Repeat("\x00", 16-len(valStr))
	return []byte(valStr), nil
}

func (f hwAddrField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 16 {
		return nil, errors.New("expected byte slice of length 16")
	}
	return string(bytes), nil
}

type serverNameField struct {
	field
}

func (f serverNameField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	} else if len(valStr) > 64 {
		return nil, errors.New("expected string of length no more than 64")
	}
	valStr += strings.Repeat("\x00", 64-len(valStr))
	return []byte(valStr), nil
}

func (f serverNameField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 64 {
		return nil, errors.New("expected byte slice of length 64")
	}
	return string(bytes), nil
}

type bootFileField struct {
	field
}

func (f bootFileField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	} else if len(valStr) > 128 {
		return nil, errors.New("expected string of length no more than 128")
	}
	valStr += strings.Repeat("\x00", 128-len(valStr))
	return []byte(valStr), nil
}

func (f bootFileField) unpack(bytes []byte) (interface{}, error) {
	if len(bytes) != 128 {
		return nil, errors.New("expected byte slice of length 128")
	}
	return string(bytes), nil
}

type ipAddressField struct {
	field
}

func (f ipAddressField) pack(value interface{}) ([]byte, error) {
	valStr, ok := value.(string)
	if !ok {
		return nil, errors.New("expected string")
	}
	return ipToBytes(valStr)
}

func (f ipAddressField) unpack(bytes []byte) (interface{}, error) {
	return bytesToIP(bytes)
}

type msgType struct {
	name        string
	optionValue uint8
}

// These are required in every DHCP packet.  Without these fields, the packet
// will not even pass dhcpPacket.isValid
var (
	fieldOp             = byteField{field{"op", 0, 1}}
	fieldHWType         = byteField{field{"htype", 1, 1}}
	fieldHWAddrLen      = byteField{field{"hlen", 2, 1}}
	fieldRelayHops      = byteField{field{"hops", 3, 1}}
	fieldTransactionID  = intField{field{"xid", 4, 4}}
	fieldTimeSinceStart = shortField{field{"secs", 8, 2}}
	fieldFlags          = shortField{field{"flags", 10, 2}}
	fieldClientIP       = ipAddressField{field{"ciaddr", 12, 4}}
	fieldYourIP         = ipAddressField{field{"yiaddr", 16, 4}}
	fieldServerIP       = ipAddressField{field{"siaddr", 20, 4}}
	fieldGatewayIP      = ipAddressField{field{"giaddr", 24, 4}}
	fieldClientHWAddr   = hwAddrField{field{"chaddr", 28, 16}}
)

// The following two fields are considered "legacy BOOTP" fields but may
// sometimes be used by DHCP clients.
var (
	fieldLegacyServerName = serverNameField{field{"servername", 44, 64}}
	fieldLegacyBootFile   = bootFileField{field{"bootfile", 108, 128}}
	fieldMagicCookie      = intField{field{"magic_cookie", 236, 4}}
)

var (
	optionTimeOffset                = intOption{option{"time_offset", 2}}
	optionRouters                   = ipListOption{option{"routers", 3}}
	optionSubnetMask                = ipAddressOption{option{"subnet_mask", 1}}
	optionTimeServers               = ipListOption{option{"time_servers", 4}}
	optionNameServers               = ipListOption{option{"name_servers", 5}}
	optionDNSServers                = ipListOption{option{"dns_servers", 6}}
	optionLogServers                = ipListOption{option{"log_servers", 7}}
	optionCookieServers             = ipListOption{option{"cookie_servers", 8}}
	optionLPRServers                = ipListOption{option{"lpr_servers", 9}}
	optionImpressServers            = ipListOption{option{"impress_servers", 10}}
	optionResourceLOCServers        = ipListOption{option{"resource_loc_servers", 11}}
	optionHostName                  = rawOption{option{"host_name", 12}}
	optionBootFileSize              = shortOption{option{"boot_file_size", 13}}
	optionMeritDumpFile             = rawOption{option{"merit_dump_file", 14}}
	optionDomainName                = rawOption{option{"domain_name", 15}}
	optionSwapServer                = ipAddressOption{option{"swap_server", 16}}
	optionRootPath                  = rawOption{option{"root_path", 17}}
	optionExtensions                = rawOption{option{"extensions", 18}}
	optionInterfaceMTU              = shortOption{option{"interface_mtu", 26}}
	optionVendorEncapsulatedOptions = rawOption{option{"vendor_encapsulated_options", 43}}
	optionRequestedIP               = ipAddressOption{option{"requested_ip", 50}}
	optionIPLeaseTime               = intOption{option{"ip_lease_time", 51}}
	optionOptionOverload            = byteOption{option{"option_overload", 52}}
	optionDHCPMessageType           = byteOption{option{"dhcp_message_type", 53}}
	optionServerID                  = ipAddressOption{option{"server_id", 54}}
	optionParameterRequestList      = byteListOption{option{"parameter_request_list", 55}}
	optionMessage                   = rawOption{option{"message", 56}}
	optionMaxDHCPMessageSize        = shortOption{option{"max_dhcp_message_size", 57}}
	optionRenewalT1TimeValue        = intOption{option{"renewal_t1_time_value", 58}}
	optionRebindingT2TimeValue      = intOption{option{"rebinding_t2_time_value", 59}}
	optionVendorID                  = rawOption{option{"vendor_id", 60}}
	optionClientID                  = rawOption{option{"client_id", 61}}
	optionTFTPServerName            = rawOption{option{"tftp_server_name", 66}}
	optionBootfileName              = rawOption{option{"bootfile_name", 67}}
	optionFullyQualifiedDomainName  = rawOption{option{"fqdn", 81}}
	optionDNSDomainSearchList       = domainListOption{option{"domain_search_list", 119}}
	optionClasslessStaticRoutes     = classlessStaticRoutesOption{option{"classless_static_routes", 121}}
	optionWebProxyAutoDiscovery     = rawOption{option{"wpad", 252}}
)

// From RFC2132, the valid DHCP message types are as follows.
var (
	msgTypeUnknown   = msgType{"UNKNOWN", 0}
	msgTypeDiscovery = msgType{"DISCOVERY", 1}
	msgTypeOffer     = msgType{"OFFER", 2}
	msgTypeRequest   = msgType{"REQUEST", 3}
	msgTypeDecline   = msgType{"DECLINE", 4}
	msgTypeAck       = msgType{"ACK", 5}
	msgTypeNAK       = msgType{"NAK", 6}
	msgTypeRelease   = msgType{"RELEASE", 7}
	msgTypeInform    = msgType{"INFORM", 8}
)

const (
	// This is per RFC 2131.  The wording doesn't seem to say that the packets
	// must be this big, but that has been the historic assumption in
	// implementations.
	dhcpMinPacketSize = 300
	ipv4NullAddress   = "0.0.0.0"

	// Unlike every other option the pad and end options are just single bytes
	// "\x00" and "\xff" (without length or data fields).
	optionPad          = 0
	optionEnd          = uint8(255)
	optionsStartOffset = 240

	// The op field in an IPv4 packet is either 1 or 2 depending on whether the
	// packet is from a server or from a client.
	fieldValueOpClientRequest  = uint8(1)
	fieldValueOpServerResponse = uint8(2)

	// 1 == 10mb ethernet hardware address type (aka MAC).
	fieldValueHWType10MBEth = uint8(1)

	// MAC addresses are still 6 bytes long.
	fieldValueHWAddrLen10MBEth = uint8(6)
	fieldValueMagicCookie      = uint32(0x63825363)
)

var (
	dhcpCommonFields = []fieldInterface{
		fieldOp,
		fieldHWType,
		fieldHWAddrLen,
		fieldRelayHops,
		fieldTransactionID,
		fieldTimeSinceStart,
		fieldFlags,
		fieldClientIP,
		fieldYourIP,
		fieldServerIP,
		fieldGatewayIP,
		fieldClientHWAddr,
	}

	dhcpRequiredFields = append(dhcpCommonFields, fieldMagicCookie)

	dhcpAllFields = append(dhcpCommonFields, []fieldInterface{fieldLegacyServerName, fieldLegacyBootFile, fieldMagicCookie}...)

	// dhcpPacketOptions are possible options that may not be in every packet.
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
	dhcpPacketOptions = []optionInterface{
		optionTimeOffset,
		optionRouters,
		optionSubnetMask,
		optionTimeServers,
		optionNameServers,
		optionDNSServers,
		optionLogServers,
		optionCookieServers,
		optionLPRServers,
		optionImpressServers,
		optionResourceLOCServers,
		optionHostName,
		optionBootFileSize,
		optionMeritDumpFile,
		optionSwapServer,
		optionDomainName,
		optionRootPath,
		optionExtensions,
		optionInterfaceMTU,
		optionVendorEncapsulatedOptions,
		optionRequestedIP,
		optionIPLeaseTime,
		optionOptionOverload,
		optionDHCPMessageType,
		optionServerID,
		optionParameterRequestList,
		optionMessage,
		optionMaxDHCPMessageSize,
		optionRenewalT1TimeValue,
		optionRebindingT2TimeValue,
		optionVendorID,
		optionClientID,
		optionTFTPServerName,
		optionBootfileName,
		optionFullyQualifiedDomainName,
		optionDNSDomainSearchList,
		optionClasslessStaticRoutes,
		optionWebProxyAutoDiscovery,
	}

	msgTypeByNum = []msgType{
		msgTypeUnknown,
		msgTypeDiscovery,
		msgTypeOffer,
		msgTypeRequest,
		msgTypeDecline,
		msgTypeAck,
		msgTypeNAK,
		msgTypeRelease,
		msgTypeInform,
	}

	optionValueParameterRequestListDefault = []uint8{
		optionRequestedIP.number(),
		optionIPLeaseTime.number(),
		optionServerID.number(),
		optionSubnetMask.number(),
		optionRouters.number(),
		optionDNSServers.number(),
		optionHostName.number(),
	}
)

func getDHCPOptionByNumber(number uint8) *optionInterface {
	for _, option := range dhcpPacketOptions {
		if option.number() == number {
			return &option
		}
	}
	return nil
}

// dhcpPacket is a class that represents a single DHCP packet and contains some
// logic to create and parse binary strings containing on the wire DHCP packets.
// While you could call |newDHCPPacket| explicitly, most users should use the
// factories to construct packets with reasonable default values in most of
// the fields, even if those values are zeros.
type dhcpPacket struct {
	options map[optionInterface]interface{}
	fields  map[fieldInterface]interface{}
}

// createDiscovery creates a discovery packet.
// Fill in fields of a DHCP packet as if it were being sent from |macAddr|.
// Requests subnet masks, broadcast addresses, router addresses, DNS addresses,
// domain search lists, client host name, and NTP server addresses. Note that
// the offer packet received in response to this packet will probably not
// contain all of that information.
func createDiscovery(macAddr string) (*dhcpPacket, error) {
	// MAC addresses are actually only 6 bytes long, however, for whatever reason,
	// DHCP allocated 12 bytes to this field.  Ease the burden on developers and
	// hide this detail.
	macAddr += strings.Repeat(string([]byte{optionPad}), 12-len(macAddr))
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpClientRequest)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, rand.Uint32())
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, ipv4NullAddress)
	packet.setField(fieldYourIP, ipv4NullAddress)
	packet.setField(fieldServerIP, ipv4NullAddress)
	packet.setField(fieldGatewayIP, ipv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, msgTypeDiscovery.optionValue)
	return packet, nil
}

// createOffer creates an offer packet, given some fields that tie the
// packet to a particular offer.
func createOffer(transactionID uint32, macAddr string, offerIP string, serverIP string) (*dhcpPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpServerResponse)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, transactionID)
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, ipv4NullAddress)
	packet.setField(fieldYourIP, offerIP)
	packet.setField(fieldServerIP, serverIP)
	packet.setField(fieldGatewayIP, ipv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, msgTypeOffer.optionValue)
	return packet, nil
}

func createRequest(transactionID uint32, macAddr string) (*dhcpPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpClientRequest)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, transactionID)
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, ipv4NullAddress)
	packet.setField(fieldYourIP, ipv4NullAddress)
	packet.setField(fieldServerIP, ipv4NullAddress)
	packet.setField(fieldGatewayIP, ipv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, msgTypeRequest.optionValue)
	return packet, nil
}

func createAck(transactionID uint32, macAddr string, grantedIP string, serverIP string) (*dhcpPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpServerResponse)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, transactionID)
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, ipv4NullAddress)
	packet.setField(fieldYourIP, grantedIP)
	packet.setField(fieldServerIP, serverIP)
	packet.setField(fieldGatewayIP, ipv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, msgTypeAck.optionValue)
	return packet, nil
}

func createNAK(transactionID uint32, macAddr string) (*dhcpPacket, error) {
	packet, err := newDHCPPacket(nil)
	if err != nil {
		return nil, err
	}
	packet.setField(fieldOp, fieldValueOpServerResponse)
	packet.setField(fieldHWType, fieldValueHWType10MBEth)
	packet.setField(fieldHWAddrLen, fieldValueHWAddrLen10MBEth)
	packet.setField(fieldRelayHops, uint8(0))
	packet.setField(fieldTransactionID, transactionID)
	packet.setField(fieldTimeSinceStart, uint16(0))
	packet.setField(fieldFlags, uint16(0))
	packet.setField(fieldClientIP, ipv4NullAddress)
	packet.setField(fieldYourIP, ipv4NullAddress)
	packet.setField(fieldServerIP, ipv4NullAddress)
	packet.setField(fieldGatewayIP, ipv4NullAddress)
	packet.setField(fieldClientHWAddr, macAddr)
	packet.setField(fieldMagicCookie, fieldValueMagicCookie)
	packet.setOption(optionDHCPMessageType, msgTypeNAK.optionValue)
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
func newDHCPPacket(bytes []byte) (*dhcpPacket, error) {
	var packet dhcpPacket
	packet.options = make(map[optionInterface]interface{})
	packet.fields = make(map[fieldInterface]interface{})
	if len(bytes) == 0 {
		return &packet, nil
	}
	if len(bytes) < optionsStartOffset+1 {
		return nil, errors.New("invalid byte string for packet")
	}
	for _, field := range dhcpAllFields {
		fieldVal, err := field.unpack(bytes[field.offset() : field.offset()+field.size()])
		if err != nil {
			return nil, err
		}
		packet.fields[field] = fieldVal
	}
	offset := optionsStartOffset
	var domainSearchListByteString []byte
	for offset < len(bytes) && bytes[offset] != optionEnd {
		dataType := bytes[offset]
		offset++
		if dataType == optionPad {
			continue
		}
		dataLength := int(bytes[offset])
		offset++
		data := bytes[offset : offset+dataLength]
		offset += dataLength
		option := getDHCPOptionByNumber(dataType)
		if option == nil {
			continue
		}
		if *option == optionDNSDomainSearchList {
			// In a cruel twist of fate, the server is allowed to give multiple
			// options with this number. The client is expected to concatenate the
			// byte strings together and use it as a single value.
			domainSearchListByteString = append(domainSearchListByteString, data...)
			continue
		}
		optionVal, err := (*option).unpack(data)
		if err != nil {
			return nil, err
		}
		optionValue := optionVal
		packet.options[*option] = optionValue
	}
	if len(domainSearchListByteString) > 0 {
		domainSearchListVal, err := optionDNSDomainSearchList.unpack(domainSearchListByteString)
		if err != nil {
			return nil, err
		}
		packet.options[optionDNSDomainSearchList] = domainSearchListVal
	}
	return &packet, nil
}

func (d *dhcpPacket) clientHWAddr() (string, error) {
	addr, ok := d.fields[fieldClientHWAddr]
	if !ok {
		return "", errors.New("client addr field not found")
	}
	addrStr, ok := addr.(string)
	if !ok {
		return "", errors.New("expected string type")
	}
	return addrStr, nil
}

// isValid checks that we have (at a minimum) values for all the required
// fields, and that the magic cookie is set correctly.
func (d *dhcpPacket) isValid() bool {
	for _, field := range dhcpRequiredFields {
		if d.fields[field] == nil {
			return false
		}
	}
	if d.fields[fieldMagicCookie] != fieldValueMagicCookie {
		return false
	}
	return true
}

// msgType gets the value of the DHCP Message Type option in this packet.
// If the option is not present, or the value of the option is not recognized,
// returns msgTypeUnknown.
func (d *dhcpPacket) msgType() (msgType, error) {
	typeNum, ok := d.options[optionDHCPMessageType]
	if !ok {
		return msgTypeUnknown, errors.New("message type option not found")
	}
	typeNumInt, ok := typeNum.(uint8)
	if !ok {
		return msgTypeUnknown, errors.New("expected uint8 type")
	}
	if typeNumInt > 0 && int(typeNumInt) < len(msgTypeByNum) {
		return msgTypeByNum[typeNumInt], nil
	}
	return msgTypeUnknown, errors.New("invalid message type")
}

func (d *dhcpPacket) txnID() (uint32, error) {
	ID, ok := d.fields[fieldTransactionID]
	if !ok {
		return 0, errors.New("transaction ID field not found")
	}
	IDInt, ok := ID.(uint32)
	if !ok {
		return 0, errors.New("expected uint32 type")
	}
	return IDInt, nil
}

func (d *dhcpPacket) getField(field fieldInterface) interface{} {
	return d.fields[field]
}

func (d *dhcpPacket) getOption(option optionInterface) interface{} {
	return d.options[option]
}

func (d *dhcpPacket) setField(field fieldInterface, fieldValue interface{}) {
	d.fields[field] = fieldValue
}

func (d *dhcpPacket) setOption(option optionInterface, optionValue interface{}) {
	d.options[option] = optionValue
}

func (d *dhcpPacket) toBinary() ([]byte, error) {
	if !d.isValid() {
		return nil, errors.New("invalid packet")
	}
	var data []byte
	var err error
	for _, field := range dhcpAllFields {
		fieldValue, ok := d.fields[field]
		if !ok {
			continue
		}
		data, err = appendField(data, field, fieldValue)
		if err != nil {
			return nil, err
		}
	}
	for _, option := range dhcpPacketOptions {
		optionValue, ok := d.options[option]
		if !ok {
			continue
		}
		data, err = appendOption(data, option, optionValue)
		if err != nil {
			return nil, err
		}
	}
	data = append(data, optionEnd)
	return append(data, bytes.Repeat([]byte{optionPad}, dhcpMinPacketSize-len(data))...), nil
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
