// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotestapp;

import android.media.AudioFormat;
import android.os.Bundle;
import android.util.Log;
import java.io.File;

/** This Chrome OS ARC++ audio sanity autotest. */
public class TestInputActivity extends MainActivity {

    private static final String TAG = "ArcAudioTest";

    public void testRecordPCM() {
        File file = null;
        SampleRecorder recorder = null;
        try {
            recorder =
                    new SampleRecorder(
                            44100, AudioFormat.ENCODING_PCM_16BIT, AudioFormat.CHANNEL_OUT_STEREO);
            file =
                    File.createTempFile(
                            "tmp", "recording.pcm", getApplicationContext().getCacheDir());
            Log.d(TAG, "start recording :" + file.getAbsolutePath());
            recorder.record(file);
            Log.d(TAG, "finish recording");
            markAsPassed();
        } catch (Exception e) {
            markAsFailed("testRecordPCM failed. " + e.toString());
        } finally {
            if (file != null && file.exists()) {
                file.delete();
            }
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
        Log.i(TAG, "start testRecordPCM");
        testRecordPCM();
        Log.i(TAG, "finish testRecordPCM");
    }
}
