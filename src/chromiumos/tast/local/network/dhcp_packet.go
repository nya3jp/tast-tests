// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"

	"chromiumos/tast/errors"
)

type OptionInterface interface {
	Pack(interface{}) (string, error)
	Unpack(string) (interface{}, error)
	name() string
	number() uint8
}

type Option struct {
	nameField   string
	numberField uint8
}

func (o Option) name() string {
	return o.nameField
}

func (o Option) number() uint8 {
	return o.numberField
}

type ByteOption struct {
	Option
}

func (o ByteOption) Pack(value interface{}) (string, error) {
	valInt, ok := value.(uint8)
	if !ok {
		return "", errors.New("expected uint8")
	}
	return string([]byte{valInt}), nil
}

func (o ByteOption) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 1 {
		return nil, errors.New("expected 1 byte")
	}
	return uint8(byteStr[0]), nil
}

type ShortOption struct {
	Option
}

func (o ShortOption) Pack(value interface{}) (string, error) {
	valInt, ok := value.(uint16)
	if !ok {
		return "", errors.New("expected uint16")
	}
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, valInt)
	return string(buf), nil
}

func (o ShortOption) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 2 {
		return nil, errors.New("expected 2 bytes")
	}
	return binary.BigEndian.Uint16([]byte(byteStr)), nil
}

type IntOption struct {
	Option
}

func (o IntOption) Pack(value interface{}) (string, error) {
	valInt, ok := value.(uint32)
	if !ok {
		return "", errors.Errorf("expected uint32 %v", value)
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, valInt)
	return string(buf), nil
}

func (o IntOption) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 4 {
		return nil, errors.New("expected 4 bytes")
	}
	return binary.BigEndian.Uint32([]byte(byteStr)), nil
}

type IPAddressOption struct {
	Option
}

func IP2Bytes(IPAddr string) (string, error) {
	IP := net.ParseIP(IPAddr)
	if IP == nil {
		return "", errors.Errorf("unable to parse IP: %s", IPAddr)
	}
	return string(IP.To4()), nil
}

func Bytes2IP(buf string) (string, error) {
	if len(buf) > 4 {
		return "", errors.Errorf("unable to parse IP: %x", buf)
	} else {
		buf += strings.Repeat("\x00", 4-len(buf))
	}
	IP := net.IP([]byte(buf))
	return IP.String(), nil
}

func (o IPAddressOption) Pack(value interface{}) (string, error) {
	valStr, ok := value.(string)
	if !ok {
		return "", errors.New("expected string")
	}
	return IP2Bytes(valStr)
}

func (o IPAddressOption) Unpack(byteStr string) (interface{}, error) {
	return Bytes2IP(byteStr)
}

type IPListOption struct {
	Option
}

func (o IPListOption) Pack(value interface{}) (string, error) {
	valSlice, ok := value.([]string)
	if !ok {
		return "", errors.New("expected string slice")
	}
	var byteStr string
	for _, addr := range valSlice {
		bytes, err := IP2Bytes(addr)
		if err != nil {
			return "", err
		}
		byteStr += bytes
	}
	return byteStr, nil
}

func (o IPListOption) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr)%4 != 0 {
		return nil, errors.New("unable to parse list")
	}
	var IPList []string
	buf := []byte(byteStr)
	for i := 0; i < len(buf); i += 4 {
		bytes, err := Bytes2IP(string(buf[i : i+4]))
		if err != nil {
			return nil, err
		}
		IPList = append(IPList, bytes)
	}
	return IPList, nil
}

type RawOption struct {
	Option
}

func (o RawOption) Pack(value interface{}) (string, error) {
	valStr, ok := value.(string)
	if !ok {
		return "", errors.New("expected string")
	}
	return valStr, nil
}

func (o RawOption) Unpack(byteStr string) (interface{}, error) {
	return byteStr, nil
}

type ByteListOption struct {
	Option
}

func (o ByteListOption) Pack(value interface{}) (string, error) {
	valBytes, ok := value.([]byte)
	if !ok {
		return "", errors.New("expected byte array")
	}
	return string(valBytes), nil
}

func (o ByteListOption) Unpack(byteStr string) (interface{}, error) {
	return []byte(byteStr), nil
}

type ClasslessStaticRoutesOption struct {
	Option
}

func (o ClasslessStaticRoutesOption) Pack(value interface{}) (string, error) {
	routeList, ok := value.([][3]interface{})
	if !ok {
		return "", errors.New("expected nx3 2-d interface{} slice")
	}
	var byteStr string
	for _, route := range routeList {
		prefixSize, ok := route[0].(uint8)
		if !ok {
			return "", errors.New("invalid prefix size")
		}
		destinationAddress, ok := route[1].(string)
		if !ok {
			return "", errors.New("invalid destination address")
		}
		routerAddress, ok := route[2].(string)
		if !ok {
			return "", errors.New("invalid router address")
		}
		byteStr += string([]byte{prefixSize})
		destinationAddressCount := (prefixSize + 7) / 8
		destinationAddressBytes, err := IP2Bytes(destinationAddress)
		if err != nil {
			return "", err
		}
		byteStr += destinationAddressBytes[:destinationAddressCount]
		routerAddressBytes, err := IP2Bytes(routerAddress)
		if err != nil {
			return "", err
		}
		byteStr += routerAddressBytes
	}
	return byteStr, nil
}

func (o ClasslessStaticRoutesOption) Unpack(byteStr string) (interface{}, error) {
	var routeList [][3]interface{}
	offset := 0
	for offset < len(byteStr) {
		prefixSize := int(byteStr[offset])
		destinationAddressCount := (prefixSize + 7) / 8
		entryEnd := offset + 1 + destinationAddressCount + 4
		if entryEnd > len(byteStr) {
			return nil, errors.New("classless domain list is corrupted")
		}
		offset++
		destinationAddressEnd := offset + destinationAddressCount
		destinationAddress, err := Bytes2IP(byteStr[offset:destinationAddressEnd])
		if err != nil {
			return nil, err
		}
		routerAddress, err := Bytes2IP(byteStr[destinationAddressEnd:entryEnd])
		if err != nil {
			return nil, err
		}
		routeList = append(routeList, [3]interface{}{uint8(prefixSize), destinationAddress, routerAddress})
		offset = entryEnd
	}
	return routeList, nil
}

type DomainListOption struct {
	Option
}

const pointerPrefix = '\xC0'

func (o DomainListOption) Pack(value interface{}) (string, error) {
	domainList, ok := value.([]string)
	if !ok {
		return "", errors.New("expected string slice")
	}
	byteStr := ""
	for _, domain := range domainList {
		for _, part := range strings.Split(domain, ".") {
			byteStr += string([]byte{uint8(len(part))})
			byteStr += part
		}
		byteStr += "\x00"
	}
	return byteStr, nil
}

func (o DomainListOption) Unpack(byteStr string) (interface{}, error) {
	var domainList []string
	offset := 0
	for offset < len(byteStr) {
		newOffset, domainParts, err := o.readDomainName(byteStr, offset)
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

func (o DomainListOption) readDomainName(byteStr string, offset int) (int, []string, error) {
	var parts []string
	for {
		if offset >= len(byteStr) {
			return 0, nil, errors.New("domain list ended without a NULL byte")
		}
		maybePartLen := int(byteStr[offset])
		offset++
		if maybePartLen == 0 {
			return offset, parts, nil
		} else if (maybePartLen & pointerPrefix) == pointerPrefix {
			if offset >= len(byteStr) {
				return 0, nil, errors.New("missing second byte of domain suffix pointer")
			}
			maybePartLen &= ^pointerPrefix
			pointerOffset := ((maybePartLen << 8) + int(byteStr[offset]))
			offset++
			_, moreParts, err := o.readDomainName(byteStr, pointerOffset)
			if err != nil {
				return 0, nil, err
			}
			parts = append(parts, moreParts...)
			return offset, parts, nil
		} else {
			partLen := maybePartLen
			if offset+partLen >= len(byteStr) {
				return 0, nil, errors.New("part of a domain goes beyond data length")
			}
			parts = append(parts, byteStr[offset:offset+partLen])
			offset += partLen
		}
	}
}

type FieldInterface interface {
	Pack(interface{}) (string, error)
	Unpack(string) (interface{}, error)
	name() string
	offset() int
	size() int
}

type Field struct {
	nameField   string
	offsetField int
	sizeField   int
}

func (f Field) name() string {
	return f.nameField
}

func (f Field) offset() int {
	return f.offsetField
}

func (f Field) size() int {
	return f.sizeField
}

type ByteField struct {
	Field
}

func (f ByteField) Pack(value interface{}) (string, error) {
	valInt, ok := value.(uint8)
	if !ok {
		return "", errors.New("expected uint8")
	}
	return string([]byte{valInt}), nil
}

func (f ByteField) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 1 {
		return nil, errors.New("expected 1 byte")
	}
	return uint8(byteStr[0]), nil
}

type ShortField struct {
	Field
}

func (f ShortField) Pack(value interface{}) (string, error) {
	valInt, ok := value.(uint16)
	if !ok {
		return "", errors.New("expected uint16")
	}
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, valInt)
	return string(buf), nil
}

func (f ShortField) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 2 {
		return nil, errors.New("expected 2 bytes")
	}
	return binary.BigEndian.Uint16([]byte(byteStr)), nil
}

type IntField struct {
	Field
}

func (f IntField) Pack(value interface{}) (string, error) {
	valInt, ok := value.(uint32)
	if !ok {
		return "", errors.Errorf("expected uint32 %v", value)
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, valInt)
	return string(buf), nil
}

func (f IntField) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 4 {
		return nil, errors.New("expected 4 bytes")
	}
	return binary.BigEndian.Uint32([]byte(byteStr)), nil
}

type HwAddrField struct {
	Field
}

func (f HwAddrField) Pack(value interface{}) (string, error) {
	valStr, ok := value.(string)
	if !ok {
		return "", errors.New("expected string")
	} else if len(valStr) > 16 {
		return "", errors.New("expected string of length no more than 16")
	}
	valStr += strings.Repeat("\x00", 16-len(valStr))
	return valStr, nil
}

func (f HwAddrField) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 16 {
		return nil, errors.New("expected string of length 16")
	}
	return byteStr, nil
}

type ServerNameField struct {
	Field
}

func (f ServerNameField) Pack(value interface{}) (string, error) {
	valStr, ok := value.(string)
	if !ok {
		return "", errors.New("expected string")
	} else if len(valStr) > 64 {
		return "", errors.New("expected string of length no more than 64")
	}
	valStr += strings.Repeat("\x00", 64-len(valStr))
	return valStr, nil
}

func (f ServerNameField) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 64 {
		return nil, errors.New("expected string of length 64")
	}
	return byteStr, nil
}

type BootFileField struct {
	Field
}

func (f BootFileField) Pack(value interface{}) (string, error) {
	valStr, ok := value.(string)
	if !ok {
		return "", errors.New("expected string")
	} else if len(valStr) > 128 {
		return "", errors.New("expected string of length no more than 128")
	}
	valStr += strings.Repeat("\x00", 128-len(valStr))
	return valStr, nil
}

func (f BootFileField) Unpack(byteStr string) (interface{}, error) {
	if len(byteStr) != 128 {
		return nil, errors.New("expected string of length 128")
	}
	return byteStr, nil
}

type IPAddressField struct {
	Field
}

func (f IPAddressField) Pack(value interface{}) (string, error) {
	valStr, ok := value.(string)
	if !ok {
		return "", errors.New("expected string")
	}
	return IP2Bytes(valStr)
}

func (f IPAddressField) Unpack(byteStr string) (interface{}, error) {
	return Bytes2IP(byteStr)
}

type MessageType struct {
	name        string
	optionValue uint8
}

var (
	FIELD_OP               = ByteField{Field{"op", 0, 1}}
	FIELD_HWTYPE           = ByteField{Field{"htype", 1, 1}}
	FIELD_HWADDR_LEN       = ByteField{Field{"hlen", 2, 1}}
	FIELD_RELAY_HOPS       = ByteField{Field{"hops", 3, 1}}
	FIELD_TRANSACTION_ID   = IntField{Field{"xid", 4, 4}}
	FIELD_TIME_SINCE_START = ShortField{Field{"secs", 8, 2}}
	FIELD_FLAGS            = ShortField{Field{"flags", 10, 2}}
	FIELD_CLIENT_IP        = IPAddressField{Field{"ciaddr", 12, 4}}
	FIELD_YOUR_IP          = IPAddressField{Field{"yiaddr", 16, 4}}
	FIELD_SERVER_IP        = IPAddressField{Field{"siaddr", 20, 4}}
	FIELD_GATEWAY_IP       = IPAddressField{Field{"giaddr", 24, 4}}
	FIELD_CLIENT_HWADDR    = HwAddrField{Field{"chaddr", 28, 16}}

	FIELD_LEGACY_SERVER_NAME = ServerNameField{Field{"servername", 44, 64}}
	FIELD_LEGACY_BOOT_FILE   = BootFileField{Field{"bootfile", 108, 128}}
	FIELD_MAGIC_COOKIE       = IntField{Field{"magic_cookie", 236, 4}}
)

var (
	OPTION_TIME_OFFSET                 = IntOption{Option{"time_offset", 2}}
	OPTION_ROUTERS                     = IPListOption{Option{"routers", 3}}
	OPTION_SUBNET_MASK                 = IPAddressOption{Option{"subnet_mask", 1}}
	OPTION_TIME_SERVERS                = IPListOption{Option{"time_servers", 4}}
	OPTION_NAME_SERVERS                = IPListOption{Option{"name_servers", 5}}
	OPTION_DNS_SERVERS                 = IPListOption{Option{"dns_servers", 6}}
	OPTION_LOG_SERVERS                 = IPListOption{Option{"log_servers", 7}}
	OPTION_COOKIE_SERVERS              = IPListOption{Option{"cookie_servers", 8}}
	OPTION_LPR_SERVERS                 = IPListOption{Option{"lpr_servers", 9}}
	OPTION_IMPRESS_SERVERS             = IPListOption{Option{"impress_servers", 10}}
	OPTION_RESOURCE_LOC_SERVERS        = IPListOption{Option{"resource_loc_servers", 11}}
	OPTION_HOST_NAME                   = RawOption{Option{"host_name", 12}}
	OPTION_BOOT_FILE_SIZE              = ShortOption{Option{"boot_file_size", 13}}
	OPTION_MERIT_DUMP_FILE             = RawOption{Option{"merit_dump_file", 14}}
	OPTION_DOMAIN_NAME                 = RawOption{Option{"domain_name", 15}}
	OPTION_SWAP_SERVER                 = IPAddressOption{Option{"swap_server", 16}}
	OPTION_ROOT_PATH                   = RawOption{Option{"root_path", 17}}
	OPTION_EXTENSIONS                  = RawOption{Option{"extensions", 18}}
	OPTION_INTERFACE_MTU               = ShortOption{Option{"interface_mtu", 26}}
	OPTION_VENDOR_ENCAPSULATED_OPTIONS = RawOption{Option{"vendor_encapsulated_options", 43}}
	OPTION_REQUESTED_IP                = IPAddressOption{Option{"requested_ip", 50}}
	OPTION_IP_LEASE_TIME               = IntOption{Option{"ip_lease_time", 51}}
	OPTION_OPTION_OVERLOAD             = ByteOption{Option{"option_overload", 52}}
	OPTION_DHCP_MESSAGE_TYPE           = ByteOption{Option{"dhcp_message_type", 53}}
	OPTION_SERVER_ID                   = IPAddressOption{Option{"server_id", 54}}
	OPTION_PARAMETER_REQUEST_LIST      = ByteListOption{Option{"parameter_request_list", 55}}
	OPTION_MESSAGE                     = RawOption{Option{"message", 56}}
	OPTION_MAX_DHCP_MESSAGE_SIZE       = ShortOption{Option{"max_dhcp_message_size", 57}}
	OPTION_RENEWAL_T1_TIME_VALUE       = IntOption{Option{"renewal_t1_time_value", 58}}
	OPTION_REBINDING_T2_TIME_VALUE     = IntOption{Option{"rebinding_t2_time_value", 59}}
	OPTION_VENDOR_ID                   = RawOption{Option{"vendor_id", 60}}
	OPTION_CLIENT_ID                   = RawOption{Option{"client_id", 61}}
	OPTION_TFTP_SERVER_NAME            = RawOption{Option{"tftp_server_name", 66}}
	OPTION_BOOTFILE_NAME               = RawOption{Option{"bootfile_name", 67}}
	OPTION_FULLY_QUALIFIED_DOMAIN_NAME = RawOption{Option{"fqdn", 81}}
	OPTION_DNS_DOMAIN_SEARCH_LIST      = DomainListOption{Option{"domain_search_list", 119}}
	OPTION_CLASSLESS_STATIC_ROUTES     = ClasslessStaticRoutesOption{Option{"classless_static_routes", 121}}
	OPTION_WEB_PROXY_AUTO_DISCOVERY    = RawOption{Option{"wpad", 252}}
)

var (
	MESSAGE_TYPE_UNKNOWN   = MessageType{"UNKNOWN", 0}
	MESSAGE_TYPE_DISCOVERY = MessageType{"DISCOVERY", 1}
	MESSAGE_TYPE_OFFER     = MessageType{"OFFER", 2}
	MESSAGE_TYPE_REQUEST   = MessageType{"REQUEST", 3}
	MESSAGE_TYPE_DECLINE   = MessageType{"DECLINE", 4}
	MESSAGE_TYPE_ACK       = MessageType{"ACK", 5}
	MESSAGE_TYPE_NAK       = MessageType{"NAK", 6}
	MESSAGE_TYPE_RELEASE   = MessageType{"RELEASE", 7}
	MESSAGE_TYPE_INFORM    = MessageType{"INFORM", 8}
)

const (
	DHCP_MIN_PACKET_SIZE = 300
	IPV4_NULL_ADDRESS    = "0.0.0.0"

	OPTION_PAD           = 0
	OPTION_END           = uint8(255)
	OPTIONS_START_OFFSET = 240

	FIELD_VALUE_OP_CLIENT_REQUEST  = uint8(1)
	FIELD_VALUE_OP_SERVER_RESPONSE = uint8(2)

	FIELD_VALUE_HWTYPE_10MB_ETH = uint8(1)

	FIELD_VALUE_HWADDR_LEN_10MB_ETH = uint8(6)
	FIELD_VALUE_MAGIC_COOKIE        = uint32(0x63825363)
)

var (
	DHCP_COMMON_FIELDS = []FieldInterface{
		FIELD_OP,
		FIELD_HWTYPE,
		FIELD_HWADDR_LEN,
		FIELD_RELAY_HOPS,
		FIELD_TRANSACTION_ID,
		FIELD_TIME_SINCE_START,
		FIELD_FLAGS,
		FIELD_CLIENT_IP,
		FIELD_YOUR_IP,
		FIELD_SERVER_IP,
		FIELD_GATEWAY_IP,
		FIELD_CLIENT_HWADDR,
	}

	DHCP_REQUIRED_FIELDS = append(DHCP_COMMON_FIELDS, FIELD_MAGIC_COOKIE)

	DHCP_ALL_FIELDS = append(DHCP_COMMON_FIELDS, []FieldInterface{FIELD_LEGACY_SERVER_NAME, FIELD_LEGACY_BOOT_FILE, FIELD_MAGIC_COOKIE}...)

	DHCP_PACKET_OPTIONS = []OptionInterface{
		OPTION_TIME_OFFSET,
		OPTION_ROUTERS,
		OPTION_SUBNET_MASK,
		OPTION_TIME_SERVERS,
		OPTION_NAME_SERVERS,
		OPTION_DNS_SERVERS,
		OPTION_LOG_SERVERS,
		OPTION_COOKIE_SERVERS,
		OPTION_LPR_SERVERS,
		OPTION_IMPRESS_SERVERS,
		OPTION_RESOURCE_LOC_SERVERS,
		OPTION_HOST_NAME,
		OPTION_BOOT_FILE_SIZE,
		OPTION_MERIT_DUMP_FILE,
		OPTION_SWAP_SERVER,
		OPTION_DOMAIN_NAME,
		OPTION_ROOT_PATH,
		OPTION_EXTENSIONS,
		OPTION_INTERFACE_MTU,
		OPTION_VENDOR_ENCAPSULATED_OPTIONS,
		OPTION_REQUESTED_IP,
		OPTION_IP_LEASE_TIME,
		OPTION_OPTION_OVERLOAD,
		OPTION_DHCP_MESSAGE_TYPE,
		OPTION_SERVER_ID,
		OPTION_PARAMETER_REQUEST_LIST,
		OPTION_MESSAGE,
		OPTION_MAX_DHCP_MESSAGE_SIZE,
		OPTION_RENEWAL_T1_TIME_VALUE,
		OPTION_REBINDING_T2_TIME_VALUE,
		OPTION_VENDOR_ID,
		OPTION_CLIENT_ID,
		OPTION_TFTP_SERVER_NAME,
		OPTION_BOOTFILE_NAME,
		OPTION_FULLY_QUALIFIED_DOMAIN_NAME,
		OPTION_DNS_DOMAIN_SEARCH_LIST,
		OPTION_CLASSLESS_STATIC_ROUTES,
		OPTION_WEB_PROXY_AUTO_DISCOVERY,
	}

	MESSAGE_TYPE_BY_NUM = []MessageType{
		MESSAGE_TYPE_UNKNOWN,
		MESSAGE_TYPE_DISCOVERY,
		MESSAGE_TYPE_OFFER,
		MESSAGE_TYPE_REQUEST,
		MESSAGE_TYPE_DECLINE,
		MESSAGE_TYPE_ACK,
		MESSAGE_TYPE_NAK,
		MESSAGE_TYPE_RELEASE,
		MESSAGE_TYPE_INFORM,
	}

	OPTION_VALUE_PARAMETER_REQUEST_LIST_DEFAULT = []uint8{
		OPTION_REQUESTED_IP.number(),
		OPTION_IP_LEASE_TIME.number(),
		OPTION_SERVER_ID.number(),
		OPTION_SUBNET_MASK.number(),
		OPTION_ROUTERS.number(),
		OPTION_DNS_SERVERS.number(),
		OPTION_HOST_NAME.number(),
	}
)

func getDHCPOptionByNumber(number uint8) *OptionInterface {
	for _, option := range DHCP_PACKET_OPTIONS {
		if option.number() == number {
			return &option
		}
	}
	return nil
}

type DHCPPacket struct {
	options map[OptionInterface]interface{}
	fields  map[FieldInterface]interface{}
}

func CreateDiscoveryPacket(macAddr string) (*DHCPPacket, error) {
	macAddr += strings.Repeat(string([]byte{OPTION_PAD}), 12-len(macAddr))
	packet, err := NewDHCPPacket("")
	if err != nil {
		return nil, err
	}
	packet.setField(FIELD_OP, FIELD_VALUE_OP_CLIENT_REQUEST)
	packet.setField(FIELD_HWTYPE, FIELD_VALUE_HWTYPE_10MB_ETH)
	packet.setField(FIELD_HWADDR_LEN, FIELD_VALUE_HWADDR_LEN_10MB_ETH)
	packet.setField(FIELD_RELAY_HOPS, uint8(0))
	packet.setField(FIELD_TRANSACTION_ID, rand.Uint32())
	packet.setField(FIELD_TIME_SINCE_START, uint16(0))
	packet.setField(FIELD_FLAGS, uint16(0))
	packet.setField(FIELD_CLIENT_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_YOUR_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_SERVER_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_GATEWAY_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_CLIENT_HWADDR, macAddr)
	packet.setField(FIELD_MAGIC_COOKIE, FIELD_VALUE_MAGIC_COOKIE)
	packet.setOption(OPTION_DHCP_MESSAGE_TYPE, MESSAGE_TYPE_DISCOVERY.optionValue)
	return packet, nil
}

func CreateOfferPacket(transactionID uint32, macAddr string, offerIP string, serverIP string) (*DHCPPacket, error) {
	packet, err := NewDHCPPacket("")
	if err != nil {
		return nil, err
	}
	packet.setField(FIELD_OP, FIELD_VALUE_OP_SERVER_RESPONSE)
	packet.setField(FIELD_HWTYPE, FIELD_VALUE_HWTYPE_10MB_ETH)
	packet.setField(FIELD_HWADDR_LEN, FIELD_VALUE_HWADDR_LEN_10MB_ETH)
	packet.setField(FIELD_RELAY_HOPS, uint8(0))
	packet.setField(FIELD_TRANSACTION_ID, transactionID)
	packet.setField(FIELD_TIME_SINCE_START, uint16(0))
	packet.setField(FIELD_FLAGS, uint16(0))
	packet.setField(FIELD_CLIENT_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_YOUR_IP, offerIP)
	packet.setField(FIELD_SERVER_IP, serverIP)
	packet.setField(FIELD_GATEWAY_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_CLIENT_HWADDR, macAddr)
	packet.setField(FIELD_MAGIC_COOKIE, FIELD_VALUE_MAGIC_COOKIE)
	packet.setOption(OPTION_DHCP_MESSAGE_TYPE, MESSAGE_TYPE_OFFER.optionValue)
	return packet, nil
}

func CreateRequestPacket(transactionID uint32, macAddr string) (*DHCPPacket, error) {
	packet, err := NewDHCPPacket("")
	if err != nil {
		return nil, err
	}
	packet.setField(FIELD_OP, FIELD_VALUE_OP_CLIENT_REQUEST)
	packet.setField(FIELD_HWTYPE, FIELD_VALUE_HWTYPE_10MB_ETH)
	packet.setField(FIELD_HWADDR_LEN, FIELD_VALUE_HWADDR_LEN_10MB_ETH)
	packet.setField(FIELD_RELAY_HOPS, uint8(0))
	packet.setField(FIELD_TRANSACTION_ID, transactionID)
	packet.setField(FIELD_TIME_SINCE_START, uint16(0))
	packet.setField(FIELD_FLAGS, uint16(0))
	packet.setField(FIELD_CLIENT_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_YOUR_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_SERVER_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_GATEWAY_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_CLIENT_HWADDR, macAddr)
	packet.setField(FIELD_MAGIC_COOKIE, FIELD_VALUE_MAGIC_COOKIE)
	packet.setOption(OPTION_DHCP_MESSAGE_TYPE, MESSAGE_TYPE_REQUEST.optionValue)
	return packet, nil
}

func CreateAcknowledgementPacket(transactionID uint32, macAddr string, grantedIP string, serverIP string) (*DHCPPacket, error) {
	packet, err := NewDHCPPacket("")
	if err != nil {
		return nil, err
	}
	packet.setField(FIELD_OP, FIELD_VALUE_OP_SERVER_RESPONSE)
	packet.setField(FIELD_HWTYPE, FIELD_VALUE_HWTYPE_10MB_ETH)
	packet.setField(FIELD_HWADDR_LEN, FIELD_VALUE_HWADDR_LEN_10MB_ETH)
	packet.setField(FIELD_RELAY_HOPS, uint8(0))
	packet.setField(FIELD_TRANSACTION_ID, transactionID)
	packet.setField(FIELD_TIME_SINCE_START, uint16(0))
	packet.setField(FIELD_FLAGS, uint16(0))
	packet.setField(FIELD_CLIENT_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_YOUR_IP, grantedIP)
	packet.setField(FIELD_SERVER_IP, serverIP)
	packet.setField(FIELD_GATEWAY_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_CLIENT_HWADDR, macAddr)
	packet.setField(FIELD_MAGIC_COOKIE, FIELD_VALUE_MAGIC_COOKIE)
	packet.setOption(OPTION_DHCP_MESSAGE_TYPE, MESSAGE_TYPE_ACK.optionValue)
	return packet, nil
}

func CreateNAKPacket(transactionID uint32, macAddr string) (*DHCPPacket, error) {
	packet, err := NewDHCPPacket("")
	if err != nil {
		return nil, err
	}
	packet.setField(FIELD_OP, FIELD_VALUE_OP_SERVER_RESPONSE)
	packet.setField(FIELD_HWTYPE, FIELD_VALUE_HWTYPE_10MB_ETH)
	packet.setField(FIELD_HWADDR_LEN, FIELD_VALUE_HWADDR_LEN_10MB_ETH)
	packet.setField(FIELD_RELAY_HOPS, uint8(0))
	packet.setField(FIELD_TRANSACTION_ID, transactionID)
	packet.setField(FIELD_TIME_SINCE_START, uint16(0))
	packet.setField(FIELD_FLAGS, uint16(0))
	packet.setField(FIELD_CLIENT_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_YOUR_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_SERVER_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_GATEWAY_IP, IPV4_NULL_ADDRESS)
	packet.setField(FIELD_CLIENT_HWADDR, macAddr)
	packet.setField(FIELD_MAGIC_COOKIE, FIELD_VALUE_MAGIC_COOKIE)
	packet.setOption(OPTION_DHCP_MESSAGE_TYPE, MESSAGE_TYPE_NAK.optionValue)
	return packet, nil
}

func NewDHCPPacket(byteStr string) (*DHCPPacket, error) {
	var packet DHCPPacket
	packet.options = make(map[OptionInterface]interface{})
	packet.fields = make(map[FieldInterface]interface{})
	if len(byteStr) == 0 {
		return &packet, nil
	}
	if len(byteStr) < OPTIONS_START_OFFSET+1 {
		return nil, errors.Errorf("invalid byte string for packet")
	}
	for _, field := range DHCP_ALL_FIELDS {
		fieldVal, err := field.Unpack(byteStr[field.offset() : field.offset()+field.size()])
		if err != nil {
			return nil, err
		}
		packet.fields[field] = fieldVal
	}
	offset := OPTIONS_START_OFFSET
	var domainSearchListByteString string
	for offset < len(byteStr) && byteStr[offset] != OPTION_END {
		dataType := byteStr[offset]
		offset++
		if dataType == OPTION_END {
			continue
		}
		dataLength := int(byteStr[offset])
		offset++
		data := byteStr[offset : offset+dataLength]
		offset += dataLength
		option := getDHCPOptionByNumber(dataType)
		if option == nil {
			// warning
			continue
		}
		if *option == OPTION_DNS_DOMAIN_SEARCH_LIST {
			domainSearchListByteString += data
			continue
		}
		optionVal, err := (*option).Unpack(data)
		if err != nil {
			return nil, err
		}
		optionValue := optionVal
		if *option == OPTION_PARAMETER_REQUEST_LIST {
			// log
		}
		packet.options[*option] = optionValue
	}
	if len(domainSearchListByteString) > 0 {
		domainSearchListVal, err := OPTION_DNS_DOMAIN_SEARCH_LIST.Unpack(domainSearchListByteString)
		if err != nil {
			return nil, err
		}
		packet.options[OPTION_DNS_DOMAIN_SEARCH_LIST] = domainSearchListVal
	}
	return &packet, nil
}

func (d *DHCPPacket) clientHWAddress() (string, error) {
	addr, ok := d.fields[FIELD_CLIENT_HWADDR]
	if !ok {
		return "", errors.New("client addr field not found")
	}
	addrStr, ok := addr.(string)
	if !ok {
		return "", errors.New("expected string type")
	}
	return addrStr, nil
}

func (d *DHCPPacket) isValid() bool {
	for _, field := range DHCP_REQUIRED_FIELDS {
		if d.fields[field] == nil {
			// warning
			return false
		}
	}
	if d.fields[FIELD_MAGIC_COOKIE] != FIELD_VALUE_MAGIC_COOKIE {
		return false
	}
	return true
}

func (d *DHCPPacket) messageType() (MessageType, error) {
	typeNum, ok := d.options[OPTION_DHCP_MESSAGE_TYPE]
	if !ok {
		return MESSAGE_TYPE_UNKNOWN, errors.New("message type option not found")
	}
	typeNumInt, ok := typeNum.(uint8)
	if !ok {
		return MESSAGE_TYPE_UNKNOWN, errors.New("expected uint8 type")
	}
	if typeNumInt > 0 && int(typeNumInt) < len(MESSAGE_TYPE_BY_NUM) {
		return MESSAGE_TYPE_BY_NUM[typeNumInt], nil
	}
	return MESSAGE_TYPE_UNKNOWN, errors.New("invalid message type")
}

func (d *DHCPPacket) transactionID() (uint32, error) {
	ID, ok := d.fields[FIELD_TRANSACTION_ID]
	if !ok {
		return 0, errors.New("transaction ID field not found")
	}
	IDInt, ok := ID.(uint32)
	if !ok {
		return 0, errors.New("expected uint32 type")
	}
	return IDInt, nil
}

func (d *DHCPPacket) getField(field FieldInterface) interface{} {
	return d.fields[field]
}

func (d *DHCPPacket) getOption(option OptionInterface) interface{} {
	return d.options[option]
}

func (d *DHCPPacket) setField(field FieldInterface, fieldValue interface{}) {
	d.fields[field] = fieldValue
}

func (d *DHCPPacket) setOption(option OptionInterface, optionValue interface{}) {
	d.options[option] = optionValue
}

func (d *DHCPPacket) toBinaryString() (string, error) {
	if !d.isValid() {
		return "", errors.New("invalid packet")
	}
	var data []string
	offset := 0
	for _, field := range DHCP_ALL_FIELDS {
		fieldValue, ok := d.fields[field]
		if !ok {
			continue
		}
		fieldData, err := field.Pack(fieldValue)
		if err != nil {
			return "", err
		}
		for offset < field.offset() {
			data = append(data, "\x00")
			offset++
		}
		data = append(data, fieldData)
		offset += field.size()
	}
	for _, option := range DHCP_PACKET_OPTIONS {
		optionValue, ok := d.options[option]
		if !ok {
			continue
		}
		serializedValue, err := option.Pack(optionValue)
		if err != nil {
			return "", err
		}
		data = append(data, string([]byte{option.number(), uint8(len(serializedValue))}))
		offset += 2
		data = append(data, serializedValue)
		offset += len(serializedValue)
	}
	data = append(data, string([]byte{OPTION_END}))
	offset++
	for offset < DHCP_MIN_PACKET_SIZE {
		data = append(data, string([]byte{OPTION_PAD}))
		offset++
	}
	return strings.Join(data, ""), nil
}

func (d *DHCPPacket) String() string {
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
