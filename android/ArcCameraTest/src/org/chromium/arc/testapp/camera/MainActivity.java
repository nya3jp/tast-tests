/*
 * Copyright 2022 The ChromiumOS Authors.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.camera;

import android.Manifest;
import android.annotation.SuppressLint;
import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.ContentValues;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.PackageManager;
import android.os.Bundle;
import android.os.Environment;
import android.provider.MediaStore;
import android.util.Log;
import android.widget.Button;
import android.widget.Toast;
import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.annotation.NonNull;
import androidx.annotation.Nullable;
import androidx.appcompat.app.AppCompatActivity;
import androidx.camera.core.CameraSelector;
import androidx.camera.core.ImageCapture;
import androidx.camera.core.ImageCaptureException;
import androidx.camera.core.Preview;
import androidx.camera.core.VideoCapture;
import androidx.camera.lifecycle.ProcessCameraProvider;
import androidx.camera.view.PreviewView;
import androidx.core.content.ContextCompat;
import androidx.lifecycle.LifecycleOwner;
import com.google.common.util.concurrent.ListenableFuture;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class MainActivity extends AppCompatActivity {
  private static final String TAG = "ARCCameraApp";
  private static final String[] PERMISSIONS = {Manifest.permission.CAMERA,
      Manifest.permission.RECORD_AUDIO, Manifest.permission.READ_EXTERNAL_STORAGE,
      Manifest.permission.WRITE_EXTERNAL_STORAGE};
  private static final String ACTION_SWITCH_CAMERA =
      "org.chromium.arc.testapp.camera.ACTION_SWITCH_CAMERA";
  private static final String ACTION_TAKE_PHOTO =
      "org.chromium.arc.testapp.camera.ACTION_TAKE_PHOTO";
  private static final String ACTION_START_RECORDING =
      "org.chromium.arc.testapp.camera.ACTION_START_RECORDING";
  private static final String ACTION_STOP_RECORDING =
      "org.chromium.arc.testapp.camera.ACTION_STOP_RECORDING";
  private static final String KEY_CAMERA_FACING =
      "org.chromium.arc.testapp.camera.KEY_CAMERA_FACING";

  private PreviewView previewView;
  private Button takePhotoButton;
  private Button recordVideoButton;
  private ImageCapture imageCapture;
  private VideoCapture videoCapture;

  private ListenableFuture<ProcessCameraProvider> cameraProviderFuture;

  /** Blocking camera operations are performed using this executor */
  private ExecutorService cameraExecutor;

  private boolean isRecording = false;

  private ActivityResultLauncher<String[]> requestPermissionLauncher = registerForActivityResult(
      new ActivityResultContracts.RequestMultiplePermissions(), isGranted -> {
        if (isGranted.containsValue(false)) {
          Log.e(TAG, "Close due to insufficient permissions: " + isGranted);
          finish();
          return;
        }
        openCamera();
      });

  private BroadcastReceiver mReceiver = new BroadcastReceiver() {
    @Override
    public void onReceive(Context context, Intent intent) {
      try {
        switch (intent.getAction()) {
          case ACTION_SWITCH_CAMERA:
            int facing = intent.getIntExtra(KEY_CAMERA_FACING, CameraSelector.LENS_FACING_FRONT);
            if (facing == CameraSelector.LENS_FACING_BACK) {
              if (!cameraProviderFuture.get().hasCamera(CameraSelector.DEFAULT_BACK_CAMERA)) {
                setResultData(Boolean.FALSE.toString());
                break;
              }
            }
            runCamera(facing);
            setResultData(Boolean.TRUE.toString());
            break;
          case ACTION_TAKE_PHOTO:
            takePhotoButton.performClick();
            break;
          case ACTION_START_RECORDING:
            if (!isRecording) {
              recordVideoButton.performClick();
            }
            break;
          case ACTION_STOP_RECORDING:
            if (isRecording) {
              recordVideoButton.performClick();
            }
            break;
        }
        setResultCode(Activity.RESULT_OK);
      } catch (Exception e) {
        setResultCode(Activity.RESULT_CANCELED);
        setResultData(e.toString());
        Log.e(TAG, "Error in " + intent.getAction(), e);
      }
    }
  };

  private static IntentFilter getFilter() {
    IntentFilter filter = new IntentFilter();
    filter.addAction(ACTION_SWITCH_CAMERA);
    filter.addAction(ACTION_TAKE_PHOTO);
    filter.addAction(ACTION_START_RECORDING);
    filter.addAction(ACTION_STOP_RECORDING);
    return filter;
  }

  @Override
  protected void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);
    setContentView(R.layout.activity_main);

    takePhotoButton = findViewById(R.id.take_photo);
    recordVideoButton = findViewById(R.id.record_video);

    // Initialize our background executor
    cameraExecutor = Executors.newSingleThreadExecutor();

    previewView = findViewById(R.id.previewView);
    cameraProviderFuture = ProcessCameraProvider.getInstance(this);
    openCameraWithPermission();

    this.registerReceiver(mReceiver, getFilter());
  }

  @Override
  protected void onDestroy() {
    super.onDestroy();

    cameraExecutor.shutdown();
    this.unregisterReceiver(mReceiver);
  }

  private void openCameraWithPermission() {
    for (String permission : PERMISSIONS) {
      if (ContextCompat.checkSelfPermission(this, permission)
          != PackageManager.PERMISSION_GRANTED) {
        requestPermissionLauncher.launch(PERMISSIONS);
        return;
      }
    }
    openCamera();
  }

  private void openCamera() {
    cameraProviderFuture.addListener(
        () -> runCamera(CameraSelector.LENS_FACING_FRONT), ContextCompat.getMainExecutor(this));

    takePhotoButton.setOnClickListener(view -> { takePhoto(); });
    recordVideoButton.setOnClickListener(view -> { recordVideo(); });
  }

  private void runCamera(int facing) {
    try {
      ProcessCameraProvider cameraProvider = cameraProviderFuture.get();
      cameraProvider.unbindAll();
      bindUseCases(cameraProvider, facing);
    } catch (ExecutionException | InterruptedException e) {
      e.printStackTrace();
    }
  }

  @SuppressLint("RestrictedApi")
  private void bindUseCases(@NonNull ProcessCameraProvider cameraProvider, int facing) {
    Preview preview = new Preview.Builder().build();
    preview.setSurfaceProvider(previewView.getSurfaceProvider());

    imageCapture = new ImageCapture.Builder()
                       .setTargetRotation(previewView.getDisplay().getRotation())
                       .build();

    videoCapture = new VideoCapture.Builder().build();

    CameraSelector cameraSelector = new CameraSelector.Builder().requireLensFacing(facing).build();
    cameraProvider.bindToLifecycle(
        (LifecycleOwner) this, cameraSelector, preview, imageCapture, videoCapture);
  }

  private void takePhoto() {
    recordVideoButton.setEnabled(false);

    long timestamp = System.currentTimeMillis();
    ContentValues values = new ContentValues();
    values.put(MediaStore.MediaColumns.DISPLAY_NAME, timestamp);
    values.put(MediaStore.MediaColumns.MIME_TYPE, "image/jpeg");
    values.put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_DCIM);
    ImageCapture.OutputFileOptions outputFileOptions =
        new ImageCapture.OutputFileOptions
            .Builder(getContentResolver(), MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values)
            .build();
    imageCapture.takePicture(
        outputFileOptions, cameraExecutor, new ImageCapture.OnImageSavedCallback() {
          @Override
          public void onImageSaved(ImageCapture.OutputFileResults outputFileResults) {
            takePhotoButton.post(() -> {
              Toast
                  .makeText(
                      getApplicationContext(), "Photo is saved successfully", Toast.LENGTH_SHORT)
                  .show();
              recordVideoButton.setEnabled(true);
            });
          }
          @Override
          public void onError(ImageCaptureException error) {
            Log.e(TAG, "Error happens when taking photo: " + error);
            error.printStackTrace();
            finish();
          }
        });
  }

  @SuppressLint({"RestrictedApi", "MissingPermission"})
  private void recordVideo() {
    if (isRecording) {
      videoCapture.stopRecording();
      isRecording = false;
      return;
    }

    isRecording = true;
    takePhotoButton.setEnabled(false);
    recordVideoButton.setText("Stop");

    long timestamp = System.currentTimeMillis();
    ContentValues values = new ContentValues();
    values.put(MediaStore.MediaColumns.DISPLAY_NAME, timestamp);
    values.put(MediaStore.MediaColumns.MIME_TYPE, "video/mp4");
    values.put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_DCIM);
    VideoCapture.OutputFileOptions outputFileOptions =
        new VideoCapture.OutputFileOptions
            .Builder(getContentResolver(), MediaStore.Video.Media.EXTERNAL_CONTENT_URI, values)
            .build();
    videoCapture.startRecording(
        outputFileOptions, cameraExecutor, new VideoCapture.OnVideoSavedCallback() {
          @Override
          public void onVideoSaved(@NonNull VideoCapture.OutputFileResults outputFileResults) {
            recordVideoButton.post(new Runnable() {
              @Override
              public void run() {
                Toast
                    .makeText(
                        getApplicationContext(), "Video is saved successfully", Toast.LENGTH_SHORT)
                    .show();
                takePhotoButton.setEnabled(true);
                recordVideoButton.setText("Record Video");
              }
            });
          }

          @Override
          public void onError(
              int videoCaptureError, @NonNull String message, @Nullable Throwable cause) {
            Log.e(TAG, "Failed to start recording: " + videoCaptureError + ", " + message);
            cause.printStackTrace();
            finish();
          }
        });
  }
}