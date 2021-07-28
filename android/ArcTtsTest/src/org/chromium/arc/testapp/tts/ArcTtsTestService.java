/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.tts;

import android.os.Environment;
import android.speech.tts.SynthesisCallback;
import android.speech.tts.SynthesisRequest;
import android.speech.tts.TextToSpeech;
import android.speech.tts.TextToSpeechService;
import android.util.Log;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;

/**
 * A text to speech engine that outputs the generated speech to a file called "ttsoutput.txt" under
 * Downloads. {@link android.speech.tts.TextToSpeechService}.
 */
public class ArcTtsTestService extends TextToSpeechService {
    static final String OUTPUT_FILENAME = "ttsoutput.txt";
    static final String TAG = "ArcTtsTestService";

    @Override
    protected String[] onGetLanguage() {
        return new String[] {"eng", "USA", ""};
    }

    @Override
    protected int onIsLanguageAvailable(String lang, String country, String variant) {
        if (lang.equals("eng")) return TextToSpeech.LANG_AVAILABLE;
        return TextToSpeech.LANG_NOT_SUPPORTED;
    }

    @Override
    protected synchronized int onLoadLanguage(String lang, String country, String variant) {
        if (lang.equals("eng")) return TextToSpeech.LANG_AVAILABLE;
        return TextToSpeech.LANG_NOT_SUPPORTED;
    }

    @Override
    protected void onStop() {
        return;
    }

    @Override
    protected synchronized void onSynthesizeText(
            SynthesisRequest request, SynthesisCallback callback) {
        String language = request.getLanguage();
        String country = request.getCountry();
        String variant = request.getVariant();
        int load = onLoadLanguage(language, country, variant);
        if (load == TextToSpeech.LANG_NOT_SUPPORTED) {
            Log.e(
                    TAG,
                    "Language Not Supported: language='"
                            + language
                            + "', country='"
                            + country
                            + "', variant='"
                            + variant
                            + "'");
            callback.error();
            return;
        }

        try {
            Path path =
                    Paths.get(
                            Environment.getExternalStoragePublicDirectory(
                                            Environment.DIRECTORY_DOWNLOADS)
                                    .getPath(),
                            OUTPUT_FILENAME);
            Files.createFile(path);
            byte[] bytes = request.getText().getBytes();
            Log.d(TAG, "Writing synthesize text to '" + path + "'");
            Files.write(
                    path,
                    bytes,
                    StandardOpenOption.WRITE,
                    StandardOpenOption.TRUNCATE_EXISTING,
                    StandardOpenOption.CREATE,
                    StandardOpenOption.SYNC);
        } catch (IOException e) {
            callback.error();
            throw new RuntimeException(e);
        }

        callback.done();
        return;
    }
}
