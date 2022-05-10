// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.media.AudioFormat;
import android.media.audiofx.AcousticEchoCanceler;
import android.os.Bundle;
import android.util.Log;
import java.io.File;

/**
 * Activity for ChromeOS ARCVM AEC audio tast.
 */
public class TestAECEffectActivity extends MainActivity {

    public void testAECRecord() {
        File file = null;
        SampleRecorder recorder = null;
        AcousticEchoCanceler aec = null;
        try {
            recorder =
                new SampleRecorder(
                    44100, AudioFormat.ENCODING_PCM_16BIT, AudioFormat.CHANNEL_OUT_STEREO);

            aec = AcousticEchoCanceler.create(recorder.getAudioSessionId());
            if (aec == null) {
                markAsFailed("could not create AcousticEchoCanceler");
                return;
            }

            Log.d(Constant.TAG, "Enabled AEC");
            aec.setEnabled(true);
            if (aec.getEnabled() == false) {
                markAsFailed("getEnabled should be true");
                return;
            }

            file =
                File.createTempFile(
                    "tmp", "recording.pcm", getApplicationContext().getCacheDir());

            Log.d(Constant.TAG, "start recording :" + file.getAbsolutePath());
            recorder.record(file);
            Log.d(Constant.TAG, "finish recording");

            aec.setEnabled(false);
            if (aec.getEnabled() != false) {
                markAsFailed("getEnabled should be true false");
                return;
            }
            Log.d(Constant.TAG, "Disabled AEC");

            markAsPassed();
        } catch (Exception e) {
            markAsFailed("testAECRecord failed. " + e.toString());
        } finally {
            if (file != null && file.exists()) {
                file.delete();
            }
            aec.release();
            recorder.release();
        }
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
    }

    @Override
    protected void onStart() {
        super.onStart();
        Log.i(Constant.TAG, "start testAECRecord");
        testAECRecord();
        Log.i(Constant.TAG, "finish testAECRecord");
    }
}
