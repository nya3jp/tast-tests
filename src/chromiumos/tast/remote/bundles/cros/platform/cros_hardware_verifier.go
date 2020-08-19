// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/descriptor"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/crypto/ssh"

	hvpb "chromiumos/hardware_verifier"
	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type stringSet map[string]struct{}
type requiredFieldSet map[string]*stringSet
type sortableMessage []proto.Message

func (set *stringSet) Contains(s string) bool {
	_, exists := (*set)[s]
	return exists
}

func (set *stringSet) Add(s string) {
	(*set)[s] = struct{}{}
}

func (s sortableMessage) Len() int {
	return len(s)
}

func (s sortableMessage) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortableMessage) Less(i, j int) bool {
	return s[i].String() < s[j].String()
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrosHardwareVerifier,
		Desc:         "Test Hardware Verifier functionality",
		Contacts:     []string{"ckclark@chromium.org", "chromeos-runtime-probe@google.com"},
		SoftwareDeps: []string{"reboot", "wilco"},
		Attr:         []string{"group:runtime_probe"},
	})
}

// CrosHardwareVerifier checks if component info are identical in three
// different sources: hardware_verifier, GenericDeviceInfo in verification
// report, and probe result of runtime_probe.
func CrosHardwareVerifier(ctx context.Context, s *testing.State) {
	fieldsMapping, err := requiredFields(ctx, s.DUT())
	if err != nil {
		s.Fatal("Cannot get GenericComponentValueAllowlists: ", err)
	}
	s.Log("ComponentValueAllowlists:", fieldsMapping)

	messagesFromProbe, err := probe(ctx, s.DUT(), fieldsMapping)
	if err != nil {
		s.Fatal("Cannot get probe results: ", err)
	}
	s.Log("MessageFromRuntimeProbe:", messagesFromProbe)

	messagesFromVerifier, err := hwVerify(ctx, s.DUT(), fieldsMapping)
	if err != nil {
		s.Fatal("Cannot get result of hardware_verifier: ", err)
	}
	s.Log("MessageFromHwVerifier:", messagesFromVerifier)

	messagesFromReport, err := report(ctx, s, fieldsMapping)
	if err != nil {
		s.Fatal("Cannot get result from report: ", err)
	}
	s.Log("MessageFromFile:", messagesFromReport)

	if diff := cmp.Diff(messagesFromReport, messagesFromVerifier); diff != "" {
		s.Log("Message mismatch (-report +hwVerifier):")
		s.Log(diff)
		s.Error("Message mismatch between report and hwVerifier (see logs for diff)")
	}

	if diff := cmp.Diff(messagesFromReport, messagesFromProbe); diff != "" {
		s.Log("Message mismatch (-report +probe):")
		s.Log(diff)
		s.Error("Message mismatch between report and probe (see logs for diff)")
	}
}

// requiredFields returns the allowed fields defined in
// /etc/hardware_verifier/hw_verification_spec.prototxt.  Probed results from
// runtime_probe will remove fields that are not allowed so that it would
// be identical with other sources.
func requiredFields(ctx context.Context, dut *dut.DUT) (requiredFieldSet, error) {
	fieldsMapping := make(requiredFieldSet)
	output, err := dut.Command("cat", "/etc/hardware_verifier/hw_verification_spec.prototxt").Output(ctx)
	if err != nil {
		return nil, err
	}

	message := &hvpb.HwVerificationSpec{}
	if err := proto.UnmarshalText(string(output), message); err != nil {
		return nil, err
	}

	for _, allowlist := range message.GenericComponentValueAllowlists {
		categorySplit := strings.Split(allowlist.ComponentCategory.String(), "_")
		for i, sp := range categorySplit {
			categorySplit[i] = strings.Title(sp)
		}
		category := strings.Join(categorySplit, "")

		for _, field := range allowlist.FieldNames {
			if m, ok := fieldsMapping[category]; ok {
				m.Add(field)
			} else {
				fieldsMapping[category] = &stringSet{field: struct{}{}}
			}
		}
	}
	return fieldsMapping, nil
}

// decodeResult will return decoded binary of hex-encoded result from
// dbus-send, also it trims the prefix, suffix, and all space characters.
// For reference, the output format of dbus-send is:
//   array of bytes [
//      1a 6f 0a ...
//   ]
func decodeResult(result string) []byte {
	result = strings.TrimSuffix(strings.TrimPrefix(result, "   array of bytes ["), "]\n")
	result = strings.NewReplacer(" ", "", "\n", "").Replace(result)
	resultBytes := []byte(result)
	decoded := make([]byte, hex.DecodedLen(len(resultBytes)))
	hex.Decode(decoded, resultBytes)
	return decoded
}

// trimFields trims fields not defined in fieldsMapping and return all
// components in a slice.  The approach is to enumerate the fields by reflect
// library and check if the names extracted by protobuf/descriptor library are
// mentioned in the fieldsMapping.
func trimFields(message *rppb.ProbeResult, fieldsMapping requiredFieldSet) (sortableMessage, error) {
	var probeResults sortableMessage
	rpmsg := reflect.ValueOf(message)
	if rpmsg.Kind() != reflect.Ptr {
		return nil, errors.New("message is not Ptr type")
	}
	rmsg := rpmsg.Elem()
	if rmsg.Kind() != reflect.Struct {
		return nil, errors.New("rmsg is not Struct type")
	}
	for category, allowlist := range fieldsMapping {
		rcomponents := rmsg.FieldByName(category)
		if rcomponents.Kind() != reflect.Slice {
			return nil, errors.New("rcomponents is not Slice type")
		}
		for i := 0; i < rcomponents.Len(); i++ {
			rpcomponent := rcomponents.Index(i)
			if rpcomponent.Kind() != reflect.Ptr {
				return nil, errors.New("rpcomponent is not Ptr type")
			}
			rcomponent := rpcomponent.Elem()
			if rcomponent.Kind() != reflect.Struct {
				return nil, errors.New("rcomponent is not Struct type")
			}
			rname := rcomponent.FieldByName("Name")
			if rname.Kind() != reflect.String {
				return nil, errors.New("rname is not String type")
			}
			// Only generic probe results are required.
			if rname.String() == "generic" {
				rpvalues := rcomponent.FieldByName("Values")
				if rpvalues.Kind() != reflect.Ptr {
					return nil, errors.New("rpvalues is not Ptr type")
				}
				if !rpvalues.IsValid() || !rpvalues.CanInterface() {
					return nil, errors.New("cannot get the value of rpvalues")
				}
				dmsg, ok := rpvalues.Interface().(descriptor.Message)
				if !ok {
					return nil, errors.New("values is not descriptor.Message")
				}
				_, desc := descriptor.ForMessage(dmsg)
				if desc == nil {
					return nil, errors.New("cannot get descriptor from message")
				}
				for j, field := range desc.GetField() {
					if field == nil {
						return nil, errors.New("field is nil")
					}
					if !allowlist.Contains(field.GetName()) {
						rvalues := rpvalues.Elem()
						if rvalues.Kind() != reflect.Struct {
							return nil, errors.New("rvalues is not Struct type")
						}
						rfield := rvalues.Field(j)
						if !rfield.CanSet() {
							return nil, errors.Errorf("cannot clear value of field %q", field.GetName())
						}
						rfield.Set(reflect.Zero(rfield.Type()))
					}
				}
				// proto.Message is embedded in descriptor.Message, so we skip the
				// check here.
				probeResults = append(probeResults, rpvalues.Interface().(proto.Message))
			}
		}
	}
	return probeResults, nil
}

// probe returns sortableMessage which collects components probed by generic
// probe statement. Also it only keep fields defined in fieldsMapping.  The
// approach to get the probe result is similar to
// https://chromium.googlesource.com/chromiumos/platform2/+/HEAD/runtime_probe/README.md#via-d_bus-call
func probe(ctx context.Context, dut *dut.DUT, fieldsMapping requiredFieldSet) (sortableMessage, error) {
	req := &rppb.ProbeRequest{ProbeDefaultCategory: true}
	b, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	decimals := make([]string, len(b))
	for i, v := range b {
		decimals[i] = strconv.Itoa(int(v))
	}
	bytesLiteral := strings.Join(decimals, ",")

	args := []string{"-u", "chronos", "dbus-send", "--system",
		"--print-reply=literal", "--type=method_call",
		"--dest=org.chromium.RuntimeProbe", "/org/chromium/RuntimeProbe",
		"org.chromium.RuntimeProbe.ProbeCategories",
		"array:byte:" + bytesLiteral,
	}

	output, err := dut.Command("sudo", args...).Output(ctx)
	if err != nil {
		return nil, err
	}
	hexEncodedResult := string(output)
	binaryResult := decodeResult(hexEncodedResult)
	message := &rppb.ProbeResult{}
	if err := proto.Unmarshal(binaryResult, message); err != nil {
		return nil, err
	}

	probeResults, err := trimFields(message, fieldsMapping)
	if err != nil {
		return nil, err
	}
	sort.Sort(probeResults)
	return probeResults, nil
}

// collectFields returns an array of {category}.Fields messages from
// GenericDeviceInfo defined in hardware_verifier.proto.  Since we could not
// guarantee the order of these messages of probe results, we just sort them by
// .String() function for comparison.
func collectFields(deviceInfo *hvpb.HwVerificationReport_GenericDeviceInfo, fieldsMapping requiredFieldSet) (sortableMessage, error) {
	var messageList sortableMessage
	rpDeviceInfo := reflect.ValueOf(deviceInfo)
	if rpDeviceInfo.Kind() != reflect.Ptr {
		return nil, errors.New("rpDeviceInfo is not Ptr type")
	}
	rDeviceInfo := rpDeviceInfo.Elem()
	if rDeviceInfo.Kind() != reflect.Struct {
		return nil, errors.New("rDeviceInfo is not Struct type")
	}

	for category := range fieldsMapping {
		fieldsList := rDeviceInfo.FieldByName(category)
		if fieldsList.Kind() != reflect.Slice {
			return nil, errors.New("fieldsList is not Slice type")
		}
		for i := 0; i < fieldsList.Len(); i++ {
			fields := fieldsList.Index(i)
			if !fields.IsValid() || !fields.CanInterface() {
				return nil, errors.New("cannot get the value of fields")
			}
			fieldsMsg, ok := fields.Interface().(proto.Message)
			if !ok {
				return nil, errors.New("fieldsMsg is not proto.Message")
			}
			messageList = append(messageList, fieldsMsg)
		}
	}
	sort.Sort(messageList)
	return messageList, nil
}

// hwVerify returns an array of {category}.Fields messages from output
// result of hardware_verifier binary.  This function is called before the
// reboot in order to verify the consistency of the execution in the init
// script /etc/init/hardware-verifier.conf.
func hwVerify(ctx context.Context, dut *dut.DUT, fieldsMapping requiredFieldSet) (sortableMessage, error) {
	args := []string{"-u", "hardware_verifier", "hardware_verifier"}
	output, err := dut.Command("sudo", args...).Output(ctx)
	if err != nil {
		exitError, isExitError := err.(*ssh.ExitError)
		// For unqualified hardware components, hardware_verifier would exit with
		// status 1 and it is expected.
		if !isExitError || exitError.ExitStatus() != 1 {
			return nil, err
		}
	}
	message := &hvpb.HwVerificationReport{}
	if err := proto.Unmarshal(output, message); err != nil {
		return nil, err
	}
	return collectFields(message.GetGenericDeviceInfo(), fieldsMapping)
}

// report returns an array of {category}.Fields messages from the
// hardware_verifier.result dumped at boot.  We remove the report file and then
// reboot and try to poll it.  Once we have the report, we will check the
// format, decode the proto message, and collect the fields required.
func report(ctx context.Context, s *testing.State, fieldsMapping requiredFieldSet) (sortableMessage, error) {
	const (
		resultFileDir              = "/var/cache"
		resultFileName             = "hardware_verifier.result"
		qualifcationStatusHeader   = "[Component Qualification Status]\n"
		genericComponentInfoHeader = "[Generic Device Info]\n"
	)
	resultFilePath := filepath.Join(resultFileDir, resultFileName)
	outPath := filepath.Join(s.OutDir(), resultFileName)

	d := s.DUT()
	s.Log("Remove result file")
	if err := d.Command("rm", "-f", resultFilePath).Run(ctx); err != nil {
		return nil, errors.Wrap(err, "cannot delete file")
	}
	s.Log("Reboot to trigger a dump of result file from hardware_verifier")
	if err := d.Reboot(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reboot DUT")
	}
	// TODO(crbug/1097710): Remove this check when this is the default behavior.
	if err := waitSystemServiceRunning(ctx, d, s); err != nil {
		return nil, err
	}
	if err := pollResultFile(ctx, d, s, resultFilePath, outPath); err != nil {
		return nil, err
	}
	s.Log("Got HwVerifier report at:", outPath)
	bytes, err := ioutil.ReadFile(outPath)
	if err != nil {
		return nil, err
	}
	fileContent := string(bytes)
	if !strings.HasPrefix(fileContent, qualifcationStatusHeader) {
		return nil, errors.New("result file format error, no qualification status")
	}
	fileContent = strings.TrimPrefix(fileContent, qualifcationStatusHeader)

	splits := strings.Split(fileContent, genericComponentInfoHeader)
	if len(splits) != 2 {
		return nil, errors.New("result file format error, no generic device info")
	}

	// We just check if it's a valid qualification status in json format.
	qualificationStatus := splits[0]
	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(qualificationStatus), &jsonResult); err != nil {
		return nil, errors.New("content is not valid JSON format")
	}

	// Check if "isCompliant" field exists and is a boolean value since we set
	// always_print_primitive_fields true while dumping.
	if val, ok := jsonResult["isCompliant"]; !ok {
		return nil, errors.New("isCompliant does not exist in qualifcation status section")
	} else if _, isBool := val.(bool); !isBool {
		return nil, errors.New("isCompliant should be a boolean value")
	}

	if err := jsonpb.UnmarshalString(qualificationStatus, &hvpb.HwVerificationReport{}); err != nil {
		return nil, errors.New("cannot decode qualification status section to a proto message")
	}

	resultText := splits[1]
	message := &hvpb.HwVerificationReport_GenericDeviceInfo{}
	if err := proto.UnmarshalText(resultText, message); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal")
	}
	return collectFields(message, fieldsMapping)
}

// pollResultFile polls the result file, and then copies it to local.
func pollResultFile(ctx context.Context, d *dut.DUT, s *testing.State, resultFilePath, outPath string) error {
	const (
		pollInterval = time.Second
		pollTimeout  = 40 * time.Second
	)

	// Current implementation is to sleep 30s before dumping result file.
	// See https://crrev.com/c/2100362 for more context.
	s.Log("Polling result file and copy it to local")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := d.GetFile(ctx, resultFilePath, outPath); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Interval: pollInterval, Timeout: pollTimeout}); err != nil {
		return errors.Wrap(err, "result file does not exist")
	}
	return nil
}

// waitSystemServiceRunning waits system-services to be start/running state.
func waitSystemServiceRunning(ctx context.Context, d *dut.DUT, s *testing.State) error {
	const (
		pollInterval = time.Second
		pollTimeout  = 2 * time.Minute
	)

	s.Log("Wait for system-services to be start/running state")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		output, err := d.Command("initctl", "status", "system-services").Output(ctx)
		if err != nil {
			return err
		}
		if strings.Contains(string(output), "start/running") {
			return nil
		}
		return errors.New("system-services is not start/running state")
	}, &testing.PollOptions{Interval: pollInterval, Timeout: pollTimeout}); err != nil {
		return err
	}
	return nil
}
