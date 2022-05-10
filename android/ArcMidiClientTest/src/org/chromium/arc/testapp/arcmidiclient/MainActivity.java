/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcmidiclient;

import android.media.midi.MidiDevice;
import android.media.midi.MidiDeviceInfo;
import android.media.midi.MidiInputPort;
import android.media.midi.MidiManager;
import android.os.Handler;
import android.os.Looper;
import android.app.Activity;
import android.os.Bundle;
import android.util.Log;

import java.io.IOException;

/**
 * This is a simple activity that is used to open the "echo" MIDI device registered with ALSA on
 * ChromeOS and write a MIDI message to it.
 *
 * <p>It is used as part of the ChromeOS ARC++ MIDI autotest, to ensure that ARC++ can access a
 * MIDI device made available through the ChromeOS MIDI daemon.
 */
public class MainActivity extends Activity {

    private static final int MIDI_CHANNEL = 3; // MIDI channels 1-16 are encoded as 0-15.
    private static final int MIDI_EVENT_NOTE_ON = 0x90;
    private static final int MIDI_MAX_VELOCITY = 127;
    private static final int MIDI_PITCH_MIDDLE_C = 60;
    private static final byte MIDI_OFFSET = 0;
    private static final String TAG = "ArcMidiTest";
    private static final String ECHO_DEVICE_NAME = "Midi Through";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        MidiManager midiManager = (MidiManager) getSystemService(MIDI_SERVICE);

        MidiDeviceInfo echoDeviceInfoTmp = null;
        for (MidiDeviceInfo info : midiManager.getDevices()) {
            Bundle properties = info.getProperties();
            String name = properties.getString(MidiDeviceInfo.PROPERTY_NAME);
            if (ECHO_DEVICE_NAME.equals(name)) {
                echoDeviceInfoTmp = info;
                break;
            }
        }

        // Device not found, this means the test will fail.
        if (echoDeviceInfoTmp == null) {
            Log.e(TAG, "Didn't find echo Midi device: " + ECHO_DEVICE_NAME);
            finish();
            return;
        }

        // Copy into a final variable to keep the compiler happy.
        final MidiDeviceInfo echoDeviceInfoFinal = echoDeviceInfoTmp;

        midiManager.openDevice(
                echoDeviceInfoFinal,
                (MidiDevice device) -> {
                    if (device == null) {
                        Log.e(TAG, "Couldn't open device: " + echoDeviceInfoFinal);
                        finish();
                        return;
                    }

                    // Get the input port.
                    MidiDeviceInfo.PortInfo portInfo = null;
                    for (MidiDeviceInfo.PortInfo curPortInfo : echoDeviceInfoFinal.getPorts()) {
                        if (curPortInfo.getType() == MidiDeviceInfo.PortInfo.TYPE_INPUT) {
                            portInfo = curPortInfo;
                            break;
                        }
                    }

                    if (portInfo == null) {
                        Log.e(
                                TAG,
                                "Couldn't find a valid input port for device: "
                                        + echoDeviceInfoFinal);
                        finish();
                        return;
                    }

                    MidiInputPort inputPort = device.openInputPort(portInfo.getPortNumber());
                    if (inputPort == null) {
                        Log.e(TAG, "Couldn't open input port: " + portInfo.getPortNumber());
                        finish();
                        return;
                    }

                    /*
                     * Encode a NoteOn message.
                     * This code block is taken from the Android developer documentation:
                     * https://developer.android.com/reference/android/media/midi/package-summary#send_a_noteon
                     */
                    byte[] buffer =
                            new byte[] {
                                (byte) (MIDI_EVENT_NOTE_ON + (MIDI_CHANNEL - 1)),
                                MIDI_PITCH_MIDDLE_C,
                                MIDI_MAX_VELOCITY,
                            };
                    Log.e(TAG, "About to send MIDI message");
                    try {
                        inputPort.send(buffer, MIDI_OFFSET, buffer.length);
                    } catch (IOException e) {
                        Log.e(TAG, "Couldn't send MIDI message for device: " + echoDeviceInfoFinal);
                        e.printStackTrace();
                        return;
                    }
                    Log.e(TAG, "Sent MIDI message");
                },
                new Handler(Looper.getMainLooper()));
    }
}
