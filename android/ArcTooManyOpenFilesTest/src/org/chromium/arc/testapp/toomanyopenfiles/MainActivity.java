/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.toomanyopenfiles;

import android.app.Activity;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;
import android.widget.Button;
import android.widget.TextView;

import java.io.IOException;
import java.nio.file.FileVisitResult;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.SimpleFileVisitor;
import java.nio.file.attribute.BasicFileAttributes;
import java.util.stream.Stream;

public class MainActivity extends Activity {
    private static final String TAG = "ArcTooManyOpenFilesTest";

    private static final String KEY_TARGET_PATH = "target_path";

    private static final String STATUS_READY = "Ready";
    private static final String STATUS_STARTED = "Started";
    private static final String STATUS_FINISHED = "Finished";
    private static final String RESULT_SUCCESS = "Success";
    private static final String RESULT_FAILURE = "Failure";

    private String mTargetPath;

    private TextView mTarget;
    private TextView mStatus;
    private TextView mResult;

    private class SimpleFileCounter extends SimpleFileVisitor<Path> {
        private long mFileCount = 0;
        public long fileCount() {
            return mFileCount;
        }

        @Override
        public FileVisitResult visitFile(Path file, BasicFileAttributes attrs) throws IOException {
            mFileCount++;
            return FileVisitResult.CONTINUE;
        }

        @Override
        public FileVisitResult visitFileFailed(Path file, IOException exc) throws IOException {
            if (exc instanceof NoSuchFileException) {
                Log.w(TAG, "File " + file + " not found");
                return FileVisitResult.CONTINUE;
            }
            Log.w(TAG, "Failed to visit file " + file);
            throw exc;
        }
    }

    private void walk() {
        mResult.setText("");
        mStatus.setText("Files.walk " + STATUS_STARTED);

        long count = -1;
        String result = RESULT_SUCCESS;
        long startTime = SystemClock.uptimeMillis();
        try (Stream<Path> walk = Files.walk(Paths.get(mTargetPath))) {
            count = walk.count();
        } catch (Exception e) {
            Log.e(TAG, "Exception thrown during Files.walk: ", e);
            result = RESULT_FAILURE + ": " + e.toString();
        }
        long elapsedTime = SystemClock.uptimeMillis() - startTime;

        Log.i(TAG, "Files.walk finished in " + elapsedTime + " ms");
        mStatus.setText("Files.walk " + STATUS_FINISHED);
        mResult.setText(result + ": " + count + " files visited in " + elapsedTime + " ms");
    }

    private void walkFileTree() {
        mResult.setText("");
        mStatus.setText("Files.walkFileTree " + STATUS_STARTED);

        String result = RESULT_SUCCESS;
        SimpleFileCounter fileCounter = new SimpleFileCounter();
        long startTime = SystemClock.uptimeMillis();
        try {
            Files.walkFileTree(Paths.get(mTargetPath), fileCounter);
        } catch (Exception e) {
            Log.e(TAG, "Exception thrown during Files.walkFileTree: ", e);
            result = RESULT_FAILURE + ": " + e.toString();
        }
        long elapsedTime = SystemClock.uptimeMillis() - startTime;

        Log.i(TAG, "Files.walkFileTree finished in " + elapsedTime + " ms");
        mStatus.setText("Files.walkFileTree " + STATUS_FINISHED);
        mResult.setText(result + ": " + fileCounter.fileCount() + " files visited in "
                + elapsedTime + " ms");
    }

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        mTargetPath = getIntent().getStringExtra(KEY_TARGET_PATH);

        mTarget = findViewById(R.id.target);
        mStatus = findViewById(R.id.status);
        mResult = findViewById(R.id.result);

        mTarget.setText(mTargetPath);

        final Button walkButton = findViewById(R.id.walk);
        walkButton.setOnClickListener(v -> {
            walk();
        });

        final Button walkFileTreeButton = findViewById(R.id.walk_file_tree);
        walkFileTreeButton.setOnClickListener(v -> {
            walkFileTree();
        });

        mStatus.setText(STATUS_READY);
    }
}
