// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.media.AudioFormat;
import android.os.Bundle;
import android.util.Log;
import java.io.File;

/**
 * Activity for ChromeOS ARC++/ARCVM audio validity tast.
 */
public class TestInputActivity extends MainActivity {

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
            Log.d(Constant.TAG, "start recording :" + file.getAbsolutePath());
            recorder.record(file);
            Log.d(Constant.TAG, "finish recording");
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
        Log.i(Constant.TAG, "start testRecordPCM");
        testRecordPCM();
        Log.i(Constant.TAG, "finish testRecordPCM");
    }
}
