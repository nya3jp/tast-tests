package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/go-vgo/robotgo"
)

const (
	KEY                    = "m"
	KEY_PRESS_COUNT        = 10
	CAMERA_STARTUP_DELAY   = 410 // Manual calibration offset
	ACTION_CAPTURE_STARTED = 1
	ACTION_CAPTURE_RESULTS = 2
)

type action struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type frameData struct {
	CornerPoints [][]point `json:"cornerPoints"`
	Line         []string
}

type hostData struct {
	FramesMetaData           []frameData `json:"framesMetaData"`
	RecordingStartTimeMillis int64       `json:"recordingStartTime"`
	VideoFps                 int64       `json:"videoFps"`
	HostSyncTimestamp        int64       `json:"hostSyncTimestamp"`
}

func Server() {
	fmt.Println("Start server...")

	ln, _ := net.Listen("tcp", "127.0.0.1:")
	_, serverPort, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Println("Listening on address:")
	fmt.Println(ln.Addr())

	openAppCommand := exec.Command("adb", "shell", "am", "start", "-n",
		"com.android.example.camera2.slowmo/com.example.android.camera2.slowmo.CameraActivity",
		"--es", "port "+serverPort)
	runOsCommand(openAppCommand)

	portForwardingCommand := exec.Command("adb", "reverse", "tcp:"+serverPort, "tcp:"+serverPort)
	runOsCommand(portForwardingCommand)

	// accept connection
	conn, _ := ln.Accept()
	fmt.Println("Got connection from:")
	fmt.Println(conn.LocalAddr().String())

	keyPressTimestamps := make([]int64, KEY_PRESS_COUNT+1)
	var testStartTimestamp int64

	hostCommunicator := json.NewDecoder(conn)

	var hostAction action

	for {
		err := hostCommunicator.Decode(&hostAction)
		fmt.Println(err)
		if hostAction.Code == ACTION_CAPTURE_STARTED {
			testStartTimestamp, keyPressTimestamps = simulateKeyPress(KEY, KEY_PRESS_COUNT)
		} else if hostAction.Code == ACTION_CAPTURE_RESULTS {
			var ocrData hostData
			hostCommunicator.Decode(&ocrData)
			calculateLag(ocrData, testStartTimestamp, keyPressTimestamps, CAMERA_STARTUP_DELAY)
		}
	}
}

func calculateLag(ocrData hostData, testStartTimestamp int64, timestamps []int64, camera_startup_delay int64) []int64 {
	lagResults := make([]int64, KEY_PRESS_COUNT)
	syncOffset := ocrData.HostSyncTimestamp - testStartTimestamp

	searchKey := ""
	for i := 0; i < len(timestamps); i++ {
		searchKey += KEY
		found := false
		for j := 0; j < len(ocrData.FramesMetaData); j++ {
			for k := 0; k < len(ocrData.FramesMetaData[j].Line); k++ {
				if strings.HasPrefix(ocrData.FramesMetaData[j].Line[k], searchKey) {
					lagResults[i] = timestamps[i] - (ocrData.RecordingStartTimeMillis + ((int64(j) * 1000) / ocrData.VideoFps)) + syncOffset - camera_startup_delay
					found = true
					break
				}
			}
			if found {
				fmt.Println("Lag = ", lagResults[i], "ms")
				break
			}
		}
	}
	return lagResults
}

func runOsCommand(cmd *exec.Cmd) int {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return 0
	}
	fmt.Println("Result: " + out.String())
	return 1
}

func simulateKeyPress(key string, keyPressCount int) (int64, []int64) {
	timestamps := make([]int64, KEY_PRESS_COUNT)
	testStartTimestamp := time.Now().UnixNano() / 1000000
	time.Sleep(1 * time.Second)
	for i := 0; i < keyPressCount; i++ {
		robotgo.TypeStr(key)
		timestamps[i] = time.Now().UnixNano() / 1000000
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("Key simulation ended")
	return testStartTimestamp, timestamps
}

func main() {
	Server()
}
