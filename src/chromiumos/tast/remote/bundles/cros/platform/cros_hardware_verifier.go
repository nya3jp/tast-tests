// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"

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

func (set *stringSet) String() string {
	keys := make([]string, 0, len(*set))
	for k := range *set {
		keys = append(keys, k)
	}
	return fmt.Sprintf("%v", keys)
}

func (s sortableMessage) Len() int {
	return len(s)
}

func (s sortableMessage) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortableMessage) Less(i, j int) bool {
	return fmt.Sprintf("%v", s[i]) < fmt.Sprintf("%v", s[j])
}

func (s sortableMessage) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("[\n")
	for i := 0; i < len(s); i++ {
		buffer.WriteString(fmt.Sprintf("%v\n", s[i]))
	}
	buffer.WriteString("]")
	return buffer.String()
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrosHardwareVerifier,
		Desc:         "Test Hardware Verifier functionality",
		Contacts:     []string{"ckclark@chromium.org", "chromeos-runtime-probe@google.com"},
		SoftwareDeps: []string{"reboot", "racc"},
		Attr:         []string{"group:runtime_probe"},
	})
}

// CrosHardwareVerifier checks if component info are identical in three
// different sources: hardware_verifier, GenericDeviceInfo in verification
// report, and probe result of runtime_probe.
func CrosHardwareVerifier(ctx context.Context, s *testing.State) {
	fieldsMapping, err := requiredFields(ctx, s)
	if err != nil {
		s.Fatal("Cannot get GenericComponentValueAllowlists: ", err)
	}
	s.Log("ComponentValueAllowlists:", fieldsMapping)

	err = waitServiceState(ctx, s.DUT(), s, "hardware_verifier", "stop/waiting")
	if err != nil {
		s.Fatal("Service hardware_verifier timed out: ", err)
	}

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

	if diff := cmp.Diff(messagesFromReport, messagesFromVerifier, protocmp.Transform()); diff != "" {
		s.Log("Message mismatch (-report +hwVerifier):")
		s.Log(diff)
		s.Error("Message mismatch between report and hwVerifier (see logs for diff)")
	}

	if diff := cmp.Diff(messagesFromReport, messagesFromProbe, protocmp.Transform()); diff != "" {
		s.Log("Message mismatch (-report +probe):")
		s.Log(diff)
		s.Error("Message mismatch between report and probe (see logs for diff)")
	}
}

// requiredFields returns the allowed fields defined in either
// /usr/local/etc/hardware_verifier/hw_verification_spec.prototxt or
// /etc/hardware_verifier/hw_verification_spec.prototxt.  Probed results from
// runtime_probe will remove fields that are not allowed so that it would
// be identical with other sources.
func requiredFields(ctx context.Context, s *testing.State) (requiredFieldSet, error) {
	const verificationSpecRelPath = "etc/hardware_verifier/hw_verification_spec.prototxt"
	dut := s.DUT()
	fieldsMapping := make(requiredFieldSet)
	// We assume that cros_debug is always enabled on testing DUTs.
	verificationSpecPath := "/usr/local/" + verificationSpecRelPath
	output, err := dut.Conn().CommandContext(ctx, "cat", verificationSpecPath).Output()
	if err != nil {
		verificationSpecPath = "/" + verificationSpecRelPath
		output, err = dut.Conn().CommandContext(ctx, "cat", verificationSpecPath).Output()
		if err != nil {
			return nil, err
		}
	}
	s.Log("Verification spec path: ", verificationSpecPath)

	message := &hvpb.HwVerificationSpec{}
	if err := proto.UnmarshalText(string(output), message); err != nil {
		return nil, err
	}

	for _, allowlist := range message.GenericComponentValueAllowlists {
		category := allowlist.ComponentCategory.String()

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
// components in a slice.  The approach is to enumerate the fields by
// protoreflect library and check if the names are mentioned in the
// fieldsMapping.
func trimFields(message *rppb.ProbeResult, fieldsMapping requiredFieldSet) (sortableMessage, error) {
	var probeResults sortableMessage
	messagePr := message.ProtoReflect()
	messageDesc := messagePr.Descriptor()
	messageFieldDescs := messageDesc.Fields()
	for category, allowlist := range fieldsMapping {
		componentListDesc := messageFieldDescs.ByName(protoreflect.Name(category))
		if componentListDesc.Message() == nil {
			return nil, errors.New("componentList is not Message type")
		}
		if !componentListDesc.IsList() {
			return nil, errors.New("componentList is not a list")
		}
		components := messagePr.Get(componentListDesc).List()
		for i := 0; i < components.Len(); i++ {
			component := components.Get(i).Message()
			compFieldsDesc := component.Descriptor().Fields()
			compNameDesc := compFieldsDesc.ByName(protoreflect.Name("name"))
			if compNameDesc.Kind() != protoreflect.StringKind {
				return nil, errors.New("compName is not a string")
			}
			compName := component.Get(compNameDesc).String()
			if compName == "generic" {
				compValuesDesc := compFieldsDesc.ByName(protoreflect.Name("values"))
				if compValuesDesc.Message() == nil {
					return nil, errors.New("compValues is not a message")
				}
				values := component.Get(compValuesDesc).Message()
				valuesFieldsDesc := values.Descriptor().Fields()
				for i := 0; i < valuesFieldsDesc.Len(); i++ {
					valuesFieldDesc := valuesFieldsDesc.Get(i)
					sp := strings.Split(string(valuesFieldDesc.FullName()), ".")
					fieldName := sp[len(sp)-1]
					if !allowlist.Contains(fieldName) {
						values.Set(valuesFieldDesc, valuesFieldDesc.Default())
					}
				}
				probeResults = append(probeResults, values.Interface().(proto.Message))
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

	output, err := dut.Conn().CommandContext(ctx, "sudo", args...).Output()
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
	deviceInfoPr := deviceInfo.ProtoReflect()
	deviceInfoDesc := deviceInfoPr.Descriptor()

	for category := range fieldsMapping {
		fieldsListDesc := deviceInfoDesc.Fields().ByName(protoreflect.Name(category))
		if fieldsListDesc.Message() == nil {
			return nil, errors.New("fieldsList is not Message type")
		}
		if !fieldsListDesc.IsList() {
			return nil, errors.New("fieldsList is not list")
		}
		fieldsList := deviceInfoPr.Get(fieldsListDesc).List()
		for i := 0; i < fieldsList.Len(); i++ {
			fields := fieldsList.Get(i).Message()
			messageList = append(messageList, fields.Interface().(proto.Message))
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
	args := []string{"-u", "hardware_verifier", "hardware_verifier", "--pii"}
	output, err := dut.Conn().CommandContext(ctx, "sudo", args...).Output()
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
	if err := d.Conn().CommandContext(ctx, "rm", "-f", resultFilePath).Run(); err != nil {
		return nil, errors.Wrap(err, "cannot delete file")
	}
	s.Log("Reboot to trigger a dump of result file from hardware_verifier")
	if err := d.Reboot(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reboot DUT")
	}
	// TODO(crbug/1097710): Remove this check when this is the default behavior.
	if err := waitServiceState(ctx, d, s, "system-services", "start/running"); err != nil {
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
		pollTimeout  = 2 * time.Minute
	)

	// Current implementation is to sleep 50s before dumping result file.
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

// waitServiceState waits for a service to be specific state.
func waitServiceState(ctx context.Context, d *dut.DUT, s *testing.State, service, state string) error {
	const (
		pollInterval = time.Second
		pollTimeout  = 2 * time.Minute
	)

	s.Logf("Wait for %s to be %s state", service, state)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		output, err := d.Conn().CommandContext(ctx, "initctl", "status", service).Output()
		if err != nil {
			return err
		}
		if strings.Contains(string(output), state) {
			return nil
		}
		return errors.Errorf("%s is not %s state", service, state)
	}, &testing.PollOptions{Interval: pollInterval, Timeout: pollTimeout}); err != nil {
		return err
	}
	return nil
}
