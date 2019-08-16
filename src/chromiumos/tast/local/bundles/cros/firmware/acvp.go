// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/firmware/third_party/boringssl/util/fipstools/acvp/acvptool/acvp"
	"chromiumos/tast/local/bundles/cros/firmware/third_party/boringssl/util/fipstools/acvp/acvptool/subprocess"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const wordLen = 4
const cr50HeaderSize = 12
const cr50RespHeaderSize = 12

//cr50ReadWriteCloser writes commands to DUT and reads the output.
//This interface is required by acvptool.
type cr50ReadWriteCloser struct {
	ctx     context.Context
	S       *testing.State
	outBuf  bytes.Buffer                              //ACVptool output goes here
	inBuf   bytes.Buffer                              //Input from ACVPtool
	parsers map[string]func([]string) (string, error) //map of functions implementing parser for each algorithm
}

//Write runs a trunks command on the DUT.
func (w *cr50ReadWriteCloser) Write(b []byte) (int, error) {
	w.inBuf.Write(b)
	cmdArg := w.getTrunksCmd()
	cmd := testexec.CommandContext(w.ctx, "trunks_send", "--raw", cmdArg)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		w.S.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	w.populateOutBuf(out)
	return 1, nil
}

//Close does nothing, but is required to implement the ReadWriteCloser interface.
func (w *cr50ReadWriteCloser) Close() error {
	return nil
}

//Read the output of the trunks command.
func (w *cr50ReadWriteCloser) Read(b []byte) (int, error) {
	return w.outBuf.Read(b)
}

// getTrunksCmd converts contents of inBuf into a trunks command.
func (w *cr50ReadWriteCloser) getTrunksCmd() string {
	args := w.parseInBuf()

	//The first index in array contains algorithm arguments are separated by '/'
	algArgs := strings.Split(args[0], "/")
	algArgs = append(algArgs, args[1:]...)
	res, err := w.parsers[algArgs[0]](algArgs[1:])
	if err != nil {
		w.S.Fatalf("Cannot parse trunks input: %v", err)
	}
	return res
}

//parseInBuf is a generic parser for ACVPTool input format.
//It returns an array of args passed.
func (w *cr50ReadWriteCloser) parseInBuf() []string {
	p := w.inBuf.Len()
	//check for error
	buf := make([]byte, p)
	w.inBuf.Read(buf)
	if len(buf) < 4 {
		w.S.Fatalf("Input buffer too short")
	}
	numArgs := binary.LittleEndian.Uint32(buf[0:4])
	var res []string
	startInd := uint32(numArgs*4 + 4)
	if uint32(len(buf)) < startInd {
		w.S.Fatalf("Input buffer too short")
	}
	//first arg is already a string
	argLen := binary.LittleEndian.Uint32(buf[4:8])
	endInd := startInd + argLen
	if uint32(len(buf)) < endInd {
		w.S.Fatalf("Input buffer too short")
	}
	res = append(res, string(buf[startInd:endInd]))
	startInd = endInd
	for i := uint32(8); i < numArgs*4+4; i += 4 {
		endInd = startInd + binary.LittleEndian.Uint32(buf[i:i+4])
		if uint32(len(buf)) < endInd {
			w.S.Fatalf("Input buffer too short")
		}
		res = append(res, hex.EncodeToString(buf[startInd:endInd]))
		startInd = endInd
	}
	return res
}

//populateOutBuf takes a cr50 command response and
//converts to output consumable by ACVPtool.
func (w *cr50ReadWriteCloser) populateOutBuf(b []byte) {
	/*# Responses to TPM vendor commands have the following header structure:
	# 8001      TPM_ST_NO_SESSIONS
	# 00000000  Response size
	# 00000000  Response code
	# 0000      Vendor Command Code*/
	b, _ = hex.DecodeString(string(b))

	respCode := binary.LittleEndian.Uint32(b[8:12])
	if respCode != 0 {
		w.S.Errorf("Unexpected response code from Cr50: %d", respCode)
	}

	respSize := uint32(len(b) - cr50RespHeaderSize)
	w.outBuf.Write([]byte{01, 00, 00, 00}) //num responses
	respSizeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(respSizeBytes, respSize)
	w.outBuf.Write(respSizeBytes)
	w.outBuf.Write(b[12:])
}

//getAESCommand constructs a trunks AES command
func getAESCommand(args []string) (string, error) {
	//Assumes args has correct size
	//Args as follows:
	/*TO DO: should only have alg args */
	//0. Alg name (AES) followed by alg args (encrypt/decrypt)
	//1. Key
	//2. PT/CT
	//3. (optional: IV)
	encrypt := false

	if args[0] == "encrypt" {
		encrypt = true
	}
	/*# Cipher modes being tested
	CIPHER_MODES = {'ECB': '00', 'CTR': '01', 'CBC': '02', 'GCM': '03', 'OFB': '04',
					'CFB': '05'}*/
	/*# 8001      TPM_ST_NO_SESSIONS
	# 00000000  Command/response size
	# 20000000  Cr50 Vendor Command (Constant, TPM Command Code)
	# 0000      Vendor Command Code (VENDOR_CC_ enum) 0000 for AES
	# Command body: test_mode|cipher_mode|
	*/
	/*
	 Command structure, shared out of band with the test driver running
	 on the host:

	 field       |    size  |              note
	 ================================================================
	 mode        |    1     | 0 - decrypt, 1 - encrypt
	 cipher_mode |    1     | as per aes_test_cipher_mode
	 key_len     |    1     | key size in bytes (16, 24 or 32)
	 key         | key len  | key to use
	 iv_len      |    1     | either 0 or 16
	 iv          | 0 or 16  | as defined by iv_len
	 aad_len     |  <= 127  | additional authentication data length
	 aad         |  aad_len | additional authentication data
	 text_len    |    2     | size of the text to process, big endian
	 text        | text_len | text to encrypt/decrypt
	*/
	var cmdBody, cmdHeader bytes.Buffer
	cmdHeader.WriteString("8001")
	if encrypt {
		cmdBody.WriteString("01")
	} else {
		cmdBody.WriteString("00")
	}
	//Assume ECB mode for now
	cmdBody.WriteString("00")
	cmdBody.WriteString(fmt.Sprintf("%02x", len(args[1])/2))
	cmdBody.WriteString(args[1])
	cmdBody.WriteString("0000")
	cmdBody.WriteString(fmt.Sprintf("%04x", len(args[2])/2))
	cmdBody.WriteString(args[2])
	cmdHeader.WriteString(fmt.Sprintf("%08x", cmdBody.Len()/2+cr50HeaderSize))
	cmdHeader.WriteString("200000000000")

	return cmdHeader.String() + cmdBody.String(), nil
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ACVP,
		Desc: "Takes a JSON generated by the ACVP server and runs the test cases in it",
		Contacts: []string{
			"gurleengrewal@chromium.org", // Test author
			"sukhomlinov@chromium.org",   // CR50 certification lead
		},
		Attr:         []string{"informational", "disabled"},
		SoftwareDeps: []string{"chrome", "tpm"},
		Data: []string{
			"aes-ecb-test.json",
		},
		Timeout: time.Hour * 10, //Tests take long
	})
}

//ACVP takes a JSON generated by the ACVP server and runs the test cases in it.
func ACVP(ctx context.Context, s *testing.State) {
	var vectors acvp.Vectors
	vectorsBytes, err := ioutil.ReadFile(s.DataPath("aes-ecb-test.json"))
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	} else {
		s.Logf("Read internal data file: %v", "aes-ecb-test.json", "\n")
	}

	if err := json.Unmarshal([]byte(vectorsBytes), &vectors); err != nil {
		s.Errorf("Failed to parse vector set: %s", err)
	}

	inout := cr50ReadWriteCloser{
		ctx: ctx,
		S:   s,
	}

	//Currently only support AES
	inout.parsers = map[string]func([]string) (string, error){
		"AES": getAESCommand,
	}
	cmd := testexec.CommandContext(ctx, "trunks_send", "--raw")

	//TO DO: cmd.Cmd should actually be something empty
	middle, err := subprocess.NewCr50(cmd.Cmd, &inout, &inout)
	if err != nil {
		s.Fatalf("failed to initialise middle: %s", err)
	}
	defer middle.Close()

	replyGroups, err := middle.Process(vectors.Algo, vectorsBytes)
	if err != nil {
		s.Errorf("failed to process middle: %s", err)
	}
	s.Logf(string(replyGroups))
}
