SELECT * FROM (
  (SELECT max(dur) AS open_device FROM slice WHERE name = 'CameraHalAdapter::OpenDevice'),
  (SELECT max(dur) AS configure_streams FROM slice WHERE name = 'CameraDeviceAdapter::ConfigureStreams')
);
