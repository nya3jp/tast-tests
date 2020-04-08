/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

// Note: This file is based on the following Android file:
// development/samples/browseable/Camera2Video/src/com.example.android.camera2video/
// Camera2VideoFragment.java

package org.chromium.arc.testapp.camerafps;

import android.app.Activity;
import android.app.Fragment;
import android.content.Context;
import android.content.res.Configuration;
import android.graphics.ImageFormat;
import android.graphics.Matrix;
import android.graphics.RectF;
import android.graphics.SurfaceTexture;
import android.hardware.camera2.CameraAccessException;
import android.hardware.camera2.CameraCaptureSession;
import android.hardware.camera2.CameraCharacteristics;
import android.hardware.camera2.CameraDevice;
import android.hardware.camera2.CameraManager;
import android.hardware.camera2.CameraMetadata;
import android.hardware.camera2.CaptureRequest;
import android.hardware.camera2.params.StreamConfigurationMap;
import android.media.Image;
import android.media.ImageReader;
import android.media.MediaRecorder;
import android.os.Bundle;
import android.os.Handler;
import android.os.HandlerThread;
import android.os.SystemClock;
import android.util.Log;
import android.util.Range;
import android.util.Size;
import android.view.LayoutInflater;
import android.view.Surface;
import android.view.TextureView;
import android.view.View;
import android.view.ViewGroup;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.ByteBuffer;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Semaphore;
import java.util.concurrent.TimeUnit;

public class Camera2VideoFragment extends Fragment {

    private static final String TAG = "ArcCameraFpsTest";

    // Default FPS for video recording. Used only if no target FPS specified.
    private static final int DEFAULT_FPS = 30;
    // Default video encoding bitrate.
    private static final int DEFAULT_VIDEO_ENCODING_BITRATE = 10000000;

    // TextureView is showing the camera preview.
    private AutoFitTextureView mTextureView;
    // Camera device (first camera).
    private CameraDevice mCameraDevice;
    // Camera preview session.
    private CameraCaptureSession mPreviewSession;
    // Size of camera preview.
    private Size mPreviewSize;
    // Size of video recording.
    private Size mVideoSize;
    // Size of camera snapshots;
    private Size mSnapshotSize;
    // MediaRecorder for recording videos.
    private MediaRecorder mMediaRecorder;
    // An additional thread for running tasks that shouldn't block the UI.
    private HandlerThread mBackgroundThread;
    // A handler for running tasks in the background.
    private Handler mBackgroundHandler;
    // A semaphore to prevent the app from exiting before closing the camera.
    private Semaphore mCameraOpenCloseLock = new Semaphore(1);
    // CaptureRequest builder for preview.
    private CaptureRequest.Builder mPreviewBuilder;
    // Surface for video recording.
    private Surface mRecorderSurface;
    // Helper objects that gathers FPS statistics.
    private CaptureCallbackHistogram mCaptureHistogram;
    // Target frames per second for preview and recording.
    private Integer mTargetFps = null;
    // Target resolution for preview and recording.
    private Size mTargetResolution = null;
    // Time when we started opening the camera.
    private long mCameraStartTime;
    // Time it took in milliseconds for opening the camera.
    private long mCameraOpenTime;
    // Time it took in milliseconds for closing the camera.
    private long mCameraCloseTime;
    // Camera characteristics contain information about supported resolutions etc.
    private CameraCharacteristics mCameraCharacteristics;

    public Camera2VideoFragment(CaptureCallbackHistogram histogram) {
        super();
        mCaptureHistogram = histogram;
    }

    // Select the largest resolution among all choices. If a specific target resolution was
    // requested, use that resolution instead if it is supported. If the requested resolution
    // is not supported, throw an error.
    private Size chooseResolution(Size[] choices) {
        long largestPixels = 0;
        Size largestSize = null;
        for (Size size : choices) {
            if (mTargetResolution != null) {
                if (size.equals(mTargetResolution)) {
                    return size;
                }
            } else {
                long pixels = size.getWidth() * size.getHeight();
                if (pixels > largestPixels) {
                    largestPixels = pixels;
                    largestSize = size;
                }
            }
        }

        if (mTargetResolution != null) {
            throw new RuntimeException("User requested resolution " + mTargetResolution.toString()
                    + " but this resolution is not supported by the camera.");
        }

        return largestSize;
    }

    public long getCameraCloseTime() {
        return mCameraCloseTime;
    }

    public long getCameraOpenTime() {
        return mCameraOpenTime;
    }

    public String getPreviewSize() {
        if (mPreviewSize != null) {
            return mPreviewSize.toString();
        } else {
            return "(null)";
        }
    }

    public String getRecordingSize() {
        if (mVideoSize != null) {
            return mVideoSize.toString();
        } else {
            return "(null)";
        }
    }

    public String getSnapshotSize() {
        if (mSnapshotSize != null) {
            return mSnapshotSize.toString();
        } else {
            return "(null)";
        }
    }

    public void setTargetFps(Integer targetFps) {
        mTargetFps = targetFps;
    }

    public void setTargetResolution(Size targetResolution) {
        mTargetResolution = targetResolution;
    }

    @Override
    public View onCreateView(
            LayoutInflater inflater, ViewGroup container, Bundle savedInstanceState) {
        return inflater.inflate(R.layout.fragment_camera2_video, container, false);
    }

    @Override
    public void onViewCreated(final View view, Bundle savedInstanceState) {
        mTextureView = (AutoFitTextureView) view.findViewById(R.id.texture);
    }

    @Override
    public void onResume() {
        super.onResume();
        startBackgroundThread();
        if (mTextureView.isAvailable()) {
            openCamera(mTextureView.getWidth(), mTextureView.getHeight());
        } else {
            mTextureView.setSurfaceTextureListener(
                    new TextureView.SurfaceTextureListener() {
                        @Override
                        public void onSurfaceTextureAvailable(
                                SurfaceTexture surfaceTexture, int width, int height) {
                            openCamera(width, height);
                        }

                        @Override
                        public void onSurfaceTextureSizeChanged(
                                SurfaceTexture surfaceTexture, int width, int height) {
                            configureTransform(width, height);
                        }

                        @Override
                        public boolean onSurfaceTextureDestroyed(SurfaceTexture surfaceTexture) {
                            return true;
                        }

                        @Override
                        public void onSurfaceTextureUpdated(SurfaceTexture surfaceTexture) {}
                    });
        }
    }

    @Override
    public void onPause() {
        closeCamera();
        stopBackgroundThread();
        super.onPause();
    }

    private void startBackgroundThread() {
        mBackgroundThread = new HandlerThread("CameraBackground");
        mBackgroundThread.start();
        mBackgroundHandler = new Handler(mBackgroundThread.getLooper());
    }

    private void stopBackgroundThread() {
        mBackgroundThread.quitSafely();
        try {
            mBackgroundThread.join();
            mBackgroundThread = null;
            mBackgroundHandler = null;
        } catch (InterruptedException e) {
            getActivity().finish();
            throw new RuntimeException(e);
        }
    }

    private <T> String getSupportedResolutions(Class<T> klass) {
        Size[] sizes = mCameraCharacteristics
                .get(CameraCharacteristics.SCALER_STREAM_CONFIGURATION_MAP)
                .getOutputSizes(klass);

        StringBuilder sb = new StringBuilder();
        sb.append("[");
        for (Size size : sizes) {
            sb.append(size.toString());
            sb.append(", ");
        }
        sb.append("]");
        return sb.toString();
    }

    public String getSnapshotResolutions() {
        return getSupportedResolutions(ImageReader.class);
    }

    public String getRecordingResolutions() {
        return getSupportedResolutions(MediaRecorder.class);
    }

    public String getPreviewResolutions() {
        return getSupportedResolutions(SurfaceTexture.class);
    }

    /** Triggers and waits for snapshot to finish. */
    public long takeCameraPicture() throws InterruptedException {
        long startTime = SystemClock.elapsedRealtime();
        final CountDownLatch latch = new CountDownLatch(1);

        try {
            // TODO: Cache map instead.
            mSnapshotSize =
                    chooseResolution(
                            mCameraCharacteristics
                                    .get(CameraCharacteristics.SCALER_STREAM_CONFIGURATION_MAP)
                                    .getOutputSizes(ImageReader.class));
            Log.i(TAG, "Taking picture: " + mSnapshotSize);
            final ImageReader reader =
                    ImageReader.newInstance(
                            mSnapshotSize.getWidth(),
                            mSnapshotSize.getHeight(),
                            ImageFormat.JPEG,
                            1 /* maximum images */);
            final CaptureRequest.Builder captureBuilder =
                    mCameraDevice.createCaptureRequest(CameraDevice.TEMPLATE_STILL_CAPTURE);
            captureBuilder.addTarget(reader.getSurface());
            captureBuilder.set(CaptureRequest.CONTROL_MODE, CameraMetadata.CONTROL_MODE_AUTO);

            String filename = getPhotoFilePath(getActivity());
            Log.i(TAG, "Saving picture in file: " + filename);

            ImageReader.OnImageAvailableListener readerListener =
                    new ImageReader.OnImageAvailableListener() {
                        @Override
                        public void onImageAvailable(ImageReader reader) {
                            final Image image = reader.acquireLatestImage();
                            ByteBuffer buffer = image.getPlanes()[0].getBuffer();
                            byte[] bytes = new byte[buffer.remaining()];
                            buffer.get(bytes);
                            File file = new File(filename);
                            FileOutputStream output = null;
                            try {
                                output = new FileOutputStream(file);
                                output.write(bytes);
                            } catch (Exception e) {
                                throw new RuntimeException(e);
                            } finally {
                                image.close();
                                try {
                                    output.close();
                                } catch (Exception e) {
                                }
                            }
                            latch.countDown();
                        }
                    };
            reader.setOnImageAvailableListener(readerListener, mBackgroundHandler);
            mCameraDevice.createCaptureSession(
                    Arrays.asList(reader.getSurface()),
                    new CameraCaptureSession.StateCallback() {
                        @Override
                        public void onConfigured(CameraCaptureSession session) {
                            try {
                                session.capture(captureBuilder.build(), null, mBackgroundHandler);
                            } catch (CameraAccessException e) {
                                throw new RuntimeException("No Camera access", e);
                            }
                        }

                        @Override
                        public void onConfigureFailed(CameraCaptureSession session) {}
                    },
                    mBackgroundHandler);
        } catch (CameraAccessException e) {
            throw new RuntimeException("No Camera access", e);
        }

        latch.await();
        return SystemClock.elapsedRealtime() - startTime;
    }

    // Open the camera device.
    private void openCamera(int width, int height) {
        final Activity activity = getActivity();
        if (null == activity || activity.isFinishing()) {
            return;
        }
        mCameraStartTime = SystemClock.elapsedRealtime();
        CameraManager manager = (CameraManager) activity.getSystemService(Context.CAMERA_SERVICE);
        try {
            if (!mCameraOpenCloseLock.tryAcquire(2500, TimeUnit.MILLISECONDS)) {
                throw new RuntimeException("Time out waiting to lock camera opening.");
            }
            String cameraId = manager.getCameraIdList()[0];

            // Choose the sizes for camera preview and video recording
            mCameraCharacteristics = manager.getCameraCharacteristics(cameraId);
            StreamConfigurationMap map =
                    mCameraCharacteristics.get(
                            CameraCharacteristics.SCALER_STREAM_CONFIGURATION_MAP);
            mVideoSize = chooseResolution(map.getOutputSizes(MediaRecorder.class));
            mPreviewSize = chooseResolution(map.getOutputSizes(SurfaceTexture.class));

            int orientation = getResources().getConfiguration().orientation;
            if (orientation == Configuration.ORIENTATION_LANDSCAPE) {
                mTextureView.setAspectRatio(mPreviewSize.getWidth(), mPreviewSize.getHeight());
            } else {
                mTextureView.setAspectRatio(mPreviewSize.getHeight(), mPreviewSize.getWidth());
            }
            configureTransform(width, height);
            mMediaRecorder = new MediaRecorder();
            manager.openCamera(
                    cameraId,
                    new CameraDevice.StateCallback() {
                        @Override
                        public void onOpened(CameraDevice cameraDevice) {
                            mCameraOpenTime = SystemClock.elapsedRealtime() - mCameraStartTime;
                            mCameraDevice = cameraDevice;
                            startPreview();
                            mCameraOpenCloseLock.release();
                            configureTransform(mTextureView.getWidth(), mTextureView.getHeight());
                        }

                        @Override
                        public void onDisconnected(CameraDevice cameraDevice) {
                            mCameraOpenCloseLock.release();
                            cameraDevice.close();
                            mCameraDevice = null;
                        }

                        @Override
                        public void onError(CameraDevice cameraDevice, int error) {
                            mCameraOpenCloseLock.release();
                            cameraDevice.close();
                            mCameraDevice = null;
                            throw new RuntimeException("Cannot open camera: Error code " + error);
                        }
                    },
                    null);
        } catch (Exception e) {
            getActivity().finish();
            throw new RuntimeException(e);
        }
    }

    // Close the camera device.
    private void closeCamera() {
        long timeBefore = SystemClock.elapsedRealtime();
        mCameraCharacteristics = null;

        try {
            mCameraOpenCloseLock.acquire();
            closePreviewSession();
            if (null != mCameraDevice) {
                mCameraDevice.close();
                mCameraDevice = null;
            }
            if (null != mMediaRecorder) {
                mMediaRecorder.release();
                mMediaRecorder = null;
            }
        } catch (Exception e) {
            getActivity().finish();
            throw new RuntimeException(e);
        } finally {
            mCameraOpenCloseLock.release();
        }

        mCameraCloseTime = SystemClock.elapsedRealtime() - timeBefore;
    }

    // Start the camera preview.
    public void startPreview() {
        try {
            closePreviewSession();
            SurfaceTexture texture = mTextureView.getSurfaceTexture();
            assert texture != null;
            texture.setDefaultBufferSize(mPreviewSize.getWidth(), mPreviewSize.getHeight());
            mPreviewBuilder = mCameraDevice.createCaptureRequest(CameraDevice.TEMPLATE_PREVIEW);

            if (mTargetFps != null) {
                mPreviewBuilder.set(
                        CaptureRequest.CONTROL_AE_TARGET_FPS_RANGE,
                        new Range<Integer>(mTargetFps, mTargetFps));
            } else {
                mPreviewBuilder.set(
                        CaptureRequest.CONTROL_AE_TARGET_FPS_RANGE,
                        new Range<Integer>(DEFAULT_FPS, DEFAULT_FPS));
            }

            Surface previewSurface = new Surface(texture);
            mPreviewBuilder.addTarget(previewSurface);

            mCameraDevice.createCaptureSession(
                    Arrays.asList(previewSurface),
                    new CameraCaptureSession.StateCallback() {
                        @Override
                        public void onConfigured(CameraCaptureSession cameraCaptureSession) {
                            mPreviewSession = cameraCaptureSession;
                            updatePreview();
                        }

                        @Override
                        public void onConfigureFailed(CameraCaptureSession cameraCaptureSession) {
                            throw new RuntimeException("Failed to configure capture session.");
                        }
                    },
                    mBackgroundHandler);
        } catch (Exception e) {
            getActivity().finish();
            throw new RuntimeException(e);
        }
    }

    // Update the camera preview. startPreview() needs to be called in advance.
    private void updatePreview() {
        try {
            mPreviewBuilder.set(CaptureRequest.CONTROL_MODE, CameraMetadata.CONTROL_MODE_AUTO);
            HandlerThread thread = new HandlerThread("CameraPreview");
            thread.start();
            mPreviewSession.setRepeatingRequest(
                    mPreviewBuilder.build(), mCaptureHistogram, mBackgroundHandler);
        } catch (Exception e) {
            getActivity().finish();
            throw new RuntimeException(e);
        }
    }

    private void closePreviewSession() {
        if (mPreviewSession != null) {
            mPreviewSession.close();
            mPreviewSession = null;
        }
    }

    // Configures the necessary Matrix transformation to `mTextureView`.
    private void configureTransform(int viewWidth, int viewHeight) {
        Activity activity = getActivity();
        if (null == mTextureView || null == mPreviewSize || null == activity) {
            return;
        }
        int rotation = activity.getWindowManager().getDefaultDisplay().getRotation();
        Matrix matrix = new Matrix();
        RectF viewRect = new RectF(0, 0, viewWidth, viewHeight);
        RectF bufferRect = new RectF(0, 0, mPreviewSize.getWidth(), mPreviewSize.getHeight());
        float centerX = viewRect.centerX();
        float centerY = viewRect.centerY();
        if (Surface.ROTATION_90 == rotation || Surface.ROTATION_270 == rotation) {
            bufferRect.offset(centerX - bufferRect.centerX(), centerY - bufferRect.centerY());
            matrix.setRectToRect(viewRect, bufferRect, Matrix.ScaleToFit.FILL);
            float scale =
                    Math.max(
                            (float) viewHeight / mPreviewSize.getHeight(),
                            (float) viewWidth / mPreviewSize.getWidth());
            matrix.postScale(scale, scale, centerX, centerY);
            matrix.postRotate(90 * (rotation - 2), centerX, centerY);
        }
        mTextureView.setTransform(matrix);
    }

    private void setUpMediaRecorder(String filename) throws IOException {
        mMediaRecorder.setAudioSource(MediaRecorder.AudioSource.MIC);
        mMediaRecorder.setVideoSource(MediaRecorder.VideoSource.SURFACE);
        mMediaRecorder.setOutputFormat(MediaRecorder.OutputFormat.MPEG_4);
        mMediaRecorder.setOutputFile(filename);
        mMediaRecorder.setVideoEncodingBitRate(DEFAULT_VIDEO_ENCODING_BITRATE);
        mMediaRecorder.setVideoFrameRate(mTargetFps == null ? DEFAULT_FPS : mTargetFps);
        mMediaRecorder.setVideoSize(mVideoSize.getWidth(), mVideoSize.getHeight());
        mMediaRecorder.setVideoEncoder(MediaRecorder.VideoEncoder.H264);
        mMediaRecorder.setAudioEncoder(MediaRecorder.AudioEncoder.AAC);
        mMediaRecorder.prepare();
    }

    private String getVideoFilePath(Context context) {
        return context.getExternalFilesDir(null).getAbsolutePath()
                + "/"
                + System.currentTimeMillis()
                + ".mp4";
    }

    private String getPhotoFilePath(Context context) {
        return context.getExternalFilesDir(null).getAbsolutePath()
                + "/"
                + System.currentTimeMillis()
                + ".jpeg";
    }

    public String startRecordingVideo() {
        try {
            closePreviewSession();

            String filename = getVideoFilePath(getActivity());
            setUpMediaRecorder(filename);

            SurfaceTexture texture = mTextureView.getSurfaceTexture();
            assert texture != null;
            texture.setDefaultBufferSize(mPreviewSize.getWidth(), mPreviewSize.getHeight());
            mPreviewBuilder = mCameraDevice.createCaptureRequest(CameraDevice.TEMPLATE_RECORD);

            // Set target FPS range
            int fps = mTargetFps == null ? DEFAULT_FPS : mTargetFps;
            mPreviewBuilder.set(
                    CaptureRequest.CONTROL_AE_TARGET_FPS_RANGE, new Range<Integer>(fps, fps));

            List<Surface> surfaces = new ArrayList<>();

            // Set up Surface for the camera preview
            Surface previewSurface = new Surface(texture);
            surfaces.add(previewSurface);
            mPreviewBuilder.addTarget(previewSurface);

            // Set up Surface for the MediaRecorder
            mRecorderSurface = mMediaRecorder.getSurface();
            surfaces.add(mRecorderSurface);
            mPreviewBuilder.addTarget(mRecorderSurface);

            // Start a capture session
            // Once the session starts, we can start recording
            mCameraDevice.createCaptureSession(
                    surfaces,
                    new CameraCaptureSession.StateCallback() {

                        @Override
                        public void onConfigured(CameraCaptureSession cameraCaptureSession) {
                            mPreviewSession = cameraCaptureSession;
                            updatePreview();
                            getActivity()
                                    .runOnUiThread(
                                            new Runnable() {
                                                @Override
                                                public void run() {
                                                    mMediaRecorder.start();
                                                }
                                            });
                        }

                        @Override
                        public void onConfigureFailed(CameraCaptureSession cameraCaptureSession) {
                            throw new RuntimeException("Failed to configure capture session.");
                        }
                    },
                    mBackgroundHandler);

            return filename;
        } catch (Exception e) {
            getActivity().finish();
            throw new RuntimeException(e);
        }
    }

    public void stopRecordingVideo() {
        mMediaRecorder.stop();
        mMediaRecorder.reset();
        startPreview();
    }
}
