/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputlatency;

import android.app.Activity;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.widget.ListView;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

/** Main activity for the ArcInputLatencyTest app. */
public class MainActivity extends Activity {
    private static final String TAG = "InputLatencyTest";
    private static final int FINISH_KEY = KeyEvent.KEYCODE_ESCAPE;
    private EventListAdapter mAdapter;
    private ListView mList;
    private ExecutorService mExecutor = Executors.newSingleThreadExecutor();

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        mAdapter = new EventListAdapter(getApplicationContext());
        mList = findViewById(R.id.event_list);
        mList.setAdapter(mAdapter);
    }

    @Override
    public void onStop() {
        // Wait up to 5 seconds for the remaining jobs in the queue to finish.
        try {
            mExecutor.awaitTermination(5, TimeUnit.SECONDS);
        } catch (InterruptedException e) {
            Log.e(
                    TAG,
                    "thread was interrupted while waiting for executor termination: "
                            + e.getMessage());
        }
        super.onStop();
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        ReceivedEvent recv =
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis());

        if (event.getKeyCode() == FINISH_KEY) {
            finish();
            return false;
        }

        traceEvent(recv);
        return super.dispatchKeyEvent(event);
    }

    @Override
    public boolean dispatchTouchEvent(MotionEvent event) {
        traceEvent(
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis()));
        return super.dispatchTouchEvent(event);
    }

    @Override
    public boolean dispatchGenericMotionEvent(MotionEvent event) {
        traceEvent(
                new ReceivedEvent(event, SystemClock.uptimeMillis(), System.currentTimeMillis()));
        return super.dispatchGenericMotionEvent(event);
    }

    /** Called to record timing of received input events. */
    private void traceEvent(ReceivedEvent recv) {
        mAdapter.add(recv);
        Log.i(TAG, recv.toString());

        // Append event to results file
        mExecutor.submit(
                () -> {
                    final Path results =
                            getExternalFilesDir(null).toPath().resolve("latency_test_results.txt");
                    final String content = recv.toString() + "\n";
                    try {
                        Files.write(
                                results,
                                content.getBytes(),
                                StandardOpenOption.CREATE,
                                StandardOpenOption.WRITE,
                                StandardOpenOption.APPEND);
                    } catch (IOException e) {
                        Log.e(
                                TAG,
                                "Failed to save latency test results to file: " + e.getMessage());
                    }
                });
    }
}
