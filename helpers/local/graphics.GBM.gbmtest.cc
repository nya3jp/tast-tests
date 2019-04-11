// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include <fcntl.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/ioctl.h>
#include <linux/dma-buf.h>

#include <base/files/scoped_file.h>
#include <base/posix/eintr_wrapper.h>
#include <base/scoped_generic.h>
#include <base/strings/stringprintf.h>
#include <gbm.h>
#include <gtest/gtest.h>
#include <xf86drm.h>
#include <xf86drmMode.h>

namespace {

constexpr size_t kBytesPerPixel = sizeof(uint32_t);

namespace internal {
struct ScopedDrmFDCloseTraits {
  static int InvalidValue() { return -1; }
  static void Free(int fd) { drmClose(fd); }
};
}  // namespace internal
using ScopedDrmFD = base::ScopedGeneric<int, internal::ScopedDrmFDCloseTraits>;

namespace internal {
struct DrmModeResourcesDeleter {
  void operator()(drmModeRes* ptr) { drmModeFreeResources(ptr); }
};
}  // namespace internal
using ScopedDrmModeResources =
    std::unique_ptr<drmModeRes, internal::DrmModeResourcesDeleter>;

namespace internal {
struct DrmModeConnectorDeleter {
  void operator()(drmModeConnector* ptr) { drmModeFreeConnector(ptr); }
};
}  // namespace internal
using ScopedDrmModeConnector =
    std::unique_ptr<drmModeConnector, internal::DrmModeConnectorDeleter>;

namespace internal {
struct GbmDeviceDeleter {
  void operator()(gbm_device* gbm) {
    if (gbm)
      gbm_device_destroy(gbm);
  }
};
}  // namespace internal
using ScopedGbmDevice =
    std::unique_ptr<gbm_device, internal::GbmDeviceDeleter>;

namespace internal {
struct GbmBoDeleter {
  void operator()(gbm_bo* bo) {
    if (bo)
      gbm_bo_destroy(bo);
  }
};
}  // namespace internal
using ScopedGbmBo = std::unique_ptr<gbm_bo, internal::GbmBoDeleter>;

class ScopedGbmBoMap {
 public:
  ScopedGbmBoMap() = default;
  ScopedGbmBoMap(void* map, gbm_bo* bo) : map_(map), bo_(bo) {}

  ~ScopedGbmBoMap() {
    if (map_)
      gbm_bo_unmap(bo_, map_);
  }

 private:
  void* map_ = nullptr;
  gbm_bo* bo_ = nullptr;
};

class ScopedMmapMemory {
 public:
  ScopedMmapMemory() = default;
  ScopedMmapMemory(void* addr, size_t length)
      : addr_(addr), length_(length) {}

  ~ScopedMmapMemory() {
    if (addr_ != MAP_FAILED)
      munmap(addr_, length_);
  }

  bool is_valid() { return addr_ != MAP_FAILED; }
  void* get() { return addr_; }

 private:
  void* addr_ = MAP_FAILED;
  size_t length_ = 0;
};

constexpr uint32_t kFormatList[] = {
  GBM_FORMAT_C8,
  GBM_FORMAT_RGB332,
  GBM_FORMAT_BGR233,
  GBM_FORMAT_XRGB4444,
  GBM_FORMAT_XBGR4444,
  GBM_FORMAT_RGBX4444,
  GBM_FORMAT_BGRX4444,
  GBM_FORMAT_ARGB4444,
  GBM_FORMAT_ABGR4444,
  GBM_FORMAT_RGBA4444,
  GBM_FORMAT_BGRA4444,
  GBM_FORMAT_XRGB1555,
  GBM_FORMAT_XBGR1555,
  GBM_FORMAT_RGBX5551,
  GBM_FORMAT_BGRX5551,
  GBM_FORMAT_ARGB1555,
  GBM_FORMAT_ABGR1555,
  GBM_FORMAT_RGBA5551,
  GBM_FORMAT_BGRA5551,
  GBM_FORMAT_RGB565,
  GBM_FORMAT_BGR565,
  GBM_FORMAT_RGB888,
  GBM_FORMAT_BGR888,
  GBM_FORMAT_XRGB8888,
  GBM_FORMAT_XBGR8888,
  GBM_FORMAT_RGBX8888,
  GBM_FORMAT_BGRX8888,
  GBM_FORMAT_ARGB8888,
  GBM_FORMAT_ABGR8888,
  GBM_FORMAT_RGBA8888,
  GBM_FORMAT_BGRA8888,
  GBM_FORMAT_XRGB2101010,
  GBM_FORMAT_XBGR2101010,
  GBM_FORMAT_RGBX1010102,
  GBM_FORMAT_BGRX1010102,
  GBM_FORMAT_ARGB2101010,
  GBM_FORMAT_ABGR2101010,
  GBM_FORMAT_RGBA1010102,
  GBM_FORMAT_BGRA1010102,
  GBM_FORMAT_YUYV,
  GBM_FORMAT_YVYU,
  GBM_FORMAT_UYVY,
  GBM_FORMAT_VYUY,
  GBM_FORMAT_AYUV,
  GBM_FORMAT_NV12,
  GBM_FORMAT_YVU420,
};

std::string FormatToString(uint32_t format) {
  switch (format) {
#define CASE(f) case f: return #f

    CASE(GBM_FORMAT_C8);
    CASE(GBM_FORMAT_RGB332);
    CASE(GBM_FORMAT_BGR233);
    CASE(GBM_FORMAT_XRGB4444);
    CASE(GBM_FORMAT_XBGR4444);
    CASE(GBM_FORMAT_RGBX4444);
    CASE(GBM_FORMAT_BGRX4444);
    CASE(GBM_FORMAT_ARGB4444);
    CASE(GBM_FORMAT_ABGR4444);
    CASE(GBM_FORMAT_RGBA4444);
    CASE(GBM_FORMAT_BGRA4444);
    CASE(GBM_FORMAT_XRGB1555);
    CASE(GBM_FORMAT_XBGR1555);
    CASE(GBM_FORMAT_RGBX5551);
    CASE(GBM_FORMAT_BGRX5551);
    CASE(GBM_FORMAT_ARGB1555);
    CASE(GBM_FORMAT_ABGR1555);
    CASE(GBM_FORMAT_RGBA5551);
    CASE(GBM_FORMAT_BGRA5551);
    CASE(GBM_FORMAT_RGB565);
    CASE(GBM_FORMAT_BGR565);
    CASE(GBM_FORMAT_RGB888);
    CASE(GBM_FORMAT_BGR888);
    CASE(GBM_FORMAT_XRGB8888);
    CASE(GBM_FORMAT_XBGR8888);
    CASE(GBM_FORMAT_RGBX8888);
    CASE(GBM_FORMAT_BGRX8888);
    CASE(GBM_FORMAT_ARGB8888);
    CASE(GBM_FORMAT_ABGR8888);
    CASE(GBM_FORMAT_RGBA8888);
    CASE(GBM_FORMAT_BGRA8888);
    CASE(GBM_FORMAT_XRGB2101010);
    CASE(GBM_FORMAT_XBGR2101010);
    CASE(GBM_FORMAT_RGBX1010102);
    CASE(GBM_FORMAT_BGRX1010102);
    CASE(GBM_FORMAT_ARGB2101010);
    CASE(GBM_FORMAT_ABGR2101010);
    CASE(GBM_FORMAT_RGBA1010102);
    CASE(GBM_FORMAT_BGRA1010102);
    CASE(GBM_FORMAT_YUYV);
    CASE(GBM_FORMAT_YVYU);
    CASE(GBM_FORMAT_UYVY);
    CASE(GBM_FORMAT_VYUY);
    CASE(GBM_FORMAT_AYUV);
    CASE(GBM_FORMAT_NV12);
    CASE(GBM_FORMAT_YVU420);
#undef CASE

    default:
      return "unknown format: " + std::to_string(format);
  }
}

constexpr uint32_t kUsageList[] = {
  GBM_BO_USE_SCANOUT,
  GBM_BO_USE_CURSOR_64X64,
  GBM_BO_USE_RENDERING,
  GBM_BO_USE_LINEAR,
  GBM_BO_USE_SW_READ_OFTEN,
  GBM_BO_USE_SW_READ_RARELY,
  GBM_BO_USE_SW_WRITE_OFTEN,
  GBM_BO_USE_SW_WRITE_RARELY,
};

std::string UsageToString(uint32_t usage) {
  switch(usage) {
#define CASE(f) case f: return #f

    CASE(GBM_BO_USE_SCANOUT);
    CASE(GBM_BO_USE_CURSOR_64X64);
    CASE(GBM_BO_USE_RENDERING);
    CASE(GBM_BO_USE_LINEAR);
    CASE(GBM_BO_USE_SW_READ_OFTEN);
    CASE(GBM_BO_USE_SW_READ_RARELY);
    CASE(GBM_BO_USE_SW_WRITE_OFTEN);
    CASE(GBM_BO_USE_SW_WRITE_RARELY);
#undef CASE

    default:
      return "unknown usage: " + std::to_string(usage);
  }
}

constexpr uint32_t kBufferList[] = {
  GBM_BO_USE_SCANOUT | GBM_BO_USE_SW_READ_RARELY | GBM_BO_USE_SW_WRITE_RARELY,
  GBM_BO_USE_RENDERING | GBM_BO_USE_SW_READ_RARELY | GBM_BO_USE_SW_WRITE_RARELY,
  GBM_BO_USE_SW_READ_RARELY | GBM_BO_USE_SW_WRITE_RARELY,
  GBM_BO_USE_SW_READ_RARELY | GBM_BO_USE_SW_WRITE_RARELY | GBM_BO_USE_TEXTURING,
  GBM_BO_USE_SW_READ_RARELY | GBM_BO_USE_SW_WRITE_RARELY | GBM_BO_USE_TEXTURING,
  GBM_BO_USE_RENDERING | GBM_BO_USE_SW_READ_RARELY |
      GBM_BO_USE_SW_WRITE_RARELY | GBM_BO_USE_TEXTURING,
  GBM_BO_USE_RENDERING | GBM_BO_USE_SCANOUT | GBM_BO_USE_SW_READ_RARELY |
      GBM_BO_USE_SW_WRITE_RARELY,
  GBM_BO_USE_RENDERING | GBM_BO_USE_SCANOUT | GBM_BO_USE_SW_READ_RARELY |
      GBM_BO_USE_SW_WRITE_RARELY | GBM_BO_USE_TEXTURING,
};

void ExpectBo(gbm_bo* bo) {
  ASSERT_TRUE(bo);
  EXPECT_GE(gbm_bo_get_width(bo), 0);
  EXPECT_GE(gbm_bo_get_height(bo), 0);
  EXPECT_GE(gbm_bo_get_stride(bo), gbm_bo_get_width(bo));

  const uint32_t format = gbm_bo_get_format(bo);

  // TODO(crbug.com/909719): Use base::ContainsValue, after uprev (which
  // supports an array).
  EXPECT_TRUE(
      std::find(std::begin(kFormatList), std::end(kFormatList), format) !=
      std::end(kFormatList)) << format;

  const size_t num_planes = gbm_bo_get_plane_count(bo);
  switch (format) {
    case GBM_FORMAT_NV12:
      EXPECT_EQ(2, num_planes);
      break;
    case GBM_FORMAT_YVU420:
      EXPECT_EQ(3, num_planes);
      break;
    default:
      EXPECT_EQ(1, num_planes);
      break;
  }

  EXPECT_EQ(gbm_bo_get_handle_for_plane(bo, 0).u32, gbm_bo_get_handle(bo).u32);

  EXPECT_EQ(0, gbm_bo_get_offset(bo, 0));
  EXPECT_GE(gbm_bo_get_plane_size(bo, 0),
            gbm_bo_get_width(bo) * gbm_bo_get_height(bo));
  EXPECT_EQ(gbm_bo_get_stride_for_plane(bo, 0), gbm_bo_get_stride(bo));

  for (size_t plane = 0; plane < num_planes; ++plane) {
    EXPECT_GT(gbm_bo_get_handle_for_plane(bo, plane).u32, 0);
    {
      base::ScopedFD fd(gbm_bo_get_plane_fd(bo, plane));
      EXPECT_TRUE(fd.is_valid());
    }
    gbm_bo_get_offset(bo, plane);  // Make sure no crash.
    EXPECT_GT(gbm_bo_get_plane_size(bo, plane), 0);
    EXPECT_GT(gbm_bo_get_stride_for_plane(bo, plane), 0);
  }
}

#define EXPECT_BO(expr) \
  EXPECT_NO_FATAL_FAILURE(ExpectBo((expr)))

// Fails with any unexpected behavior for the given bo.
#define ASSERT_BO(expr) \
  do { \
    ExpectBo((expr)); \
    ASSERT_FALSE(::testing::Test::HasFailure()); \
  } while(0)

bool HasConnectedConnector(int fd, const drmModeRes* resources) {
  for (int i = 0; i < resources->count_connectors; ++i) {
    ScopedDrmModeConnector connector(
        drmModeGetConnector(fd, resources->connectors[i]));
    if (!connector)
      continue;
    if (connector->count_modes > 0 &&
        connector->connection == DRM_MODE_CONNECTED) {
      return true;
    }
  }
  return false;
}

ScopedDrmFD DrmOpen() {
  // Find the first drm device with a connected display.
  for (int i = 0; i < DRM_MAX_MINOR; ++i) {
    auto dev_name = base::StringPrintf(DRM_DEV_NAME, DRM_DIR_NAME, i);
    ScopedDrmFD fd(HANDLE_EINTR(open(dev_name.c_str(), O_RDWR)));
    if (!fd.is_valid())
      continue;

    ScopedDrmModeResources resources(drmModeGetResources(fd.get()));
    if (!resources)
      continue;

    if (resources->count_crtcs > 0 &&
        HasConnectedConnector(fd.get(), resources.get())) {
      return fd;
    }
  }

  // If no drm device has a connected display, fall back to the first
  // drm device.
  for (int i = 0; i < DRM_MAX_MINOR; ++i) {
    auto dev_name = base::StringPrintf(DRM_DEV_NAME, DRM_DIR_NAME, i);
    ScopedDrmFD fd(HANDLE_EINTR(open(dev_name.c_str(), O_RDWR)));
    if (fd.is_valid())
      return fd;
  }

  return {};
}

base::ScopedFD DrmOpenVgem() {
  for (int i = 0; i < 16; ++i) {
    struct stat st;
    auto sys_card_path =
        base::StringPrintf("/sys/bus/platform/devices/vgem/drm/card%d", i);
    if (stat(sys_card_path.c_str(), &st) == 0) {
      auto dev_card_path = base::StringPrintf("/dev/dri/card%d", i);
      return base::ScopedFD(HANDLE_EINTR(open(dev_card_path.c_str(), O_RDWR)));
    }
  }

  return {};
}

int CreateVgemBo(int fd, size_t size, uint32_t* handle) {
  struct drm_mode_create_dumb create = {};
  create.height = size;
  create.width = 1;
  create.bpp = 8;

  int ret = drmIoctl(fd, DRM_IOCTL_MODE_CREATE_DUMB, &create);
  if (ret)
    return ret;

  CHECK_GE(create.size, size);
  *handle = create.handle;
  return 0;
}

class GraphicsGbmTest : public testing::Test {
 public:
  GraphicsGbmTest() : fd_(DrmOpen()), gbm_(gbm_create_device(fd_.get())) {}
  ~GraphicsGbmTest() = default;

  void SetUp() override {
    ASSERT_TRUE(fd_.is_valid());
    ASSERT_TRUE(gbm_.get());
  }

 protected:
  ScopedDrmFD fd_;
  ScopedGbmDevice gbm_;
};

TEST_F(GraphicsGbmTest, BackendName) {
  EXPECT_TRUE(gbm_device_get_backend_name(gbm_.get()));
}

TEST_F(GraphicsGbmTest, Reinit) {
  gbm_.reset();
  fd_.reset();

  fd_ = DrmOpen();
  ASSERT_TRUE(fd_.is_valid());
  gbm_.reset(gbm_create_device(fd_.get()));
  ASSERT_TRUE(gbm_.get());

  EXPECT_TRUE(gbm_device_get_backend_name(gbm_.get()));

  ScopedGbmBo bo(gbm_bo_create(
      gbm_.get(), 1024, 1024, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
  EXPECT_BO(bo.get());
}

// Tests repeated alloc/free.
TEST_F(GraphicsGbmTest, AllocFree) {
  for (int i = 0; i < 1000; ++i) {
    ScopedGbmBo bo(gbm_bo_create(
        gbm_.get(), 1024, 1024, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
    EXPECT_BO(bo.get());
  }
}

// Tests that we can allocate different buffer dimensions.
TEST_F(GraphicsGbmTest, AllocFreeSizes) {
  // Test i * i size.
  for (int i = 1; i < 1920; ++i) {
    SCOPED_TRACE("i: " + std::to_string(i));
    ScopedGbmBo bo(gbm_bo_create(
        gbm_.get(), i, i, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
    EXPECT_BO(bo.get());
  }

  // Test i * 1 size.
  for (int i = 1; i < 1920; ++i) {
    SCOPED_TRACE("size: " + std::to_string(i) + " x 1");
    ScopedGbmBo bo(gbm_bo_create(
        gbm_.get(), i, 1, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
    EXPECT_BO(bo.get());
  }

  // Test 1 * i size.
  for (int i = 1; i < 1920; ++i) {
    SCOPED_TRACE("size: 1 x " + std::to_string(i));
    ScopedGbmBo bo(gbm_bo_create(
        gbm_.get(), 1, i, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
    EXPECT_BO(bo.get());
  }
}

// Tests that we can allocate different buffer formats.
TEST_F(GraphicsGbmTest, AllocFreeFormats) {
  for (const auto format : kFormatList) {
    if (!gbm_device_is_format_supported(
            gbm_.get(), format, GBM_BO_USE_RENDERING)) {
      continue;
    }
    SCOPED_TRACE("Format: " + FormatToString(format));
    ScopedGbmBo bo(gbm_bo_create(
        gbm_.get(), 1024, 1024, format, GBM_BO_USE_RENDERING));
    EXPECT_BO(bo.get());
  }
}

// Tests that we find at least one working format for each usage.
TEST_F(GraphicsGbmTest, AllocFreeUsage) {
  for (const auto usage : kUsageList) {
    SCOPED_TRACE("Usage: " + UsageToString(usage));
    bool found = false;
    const uint32_t size = usage == GBM_BO_USE_CURSOR_64X64 ? 64 : 1024;
    for (const auto format : kFormatList) {
      if (!gbm_device_is_format_supported(gbm_.get(), format, usage))
        continue;
      SCOPED_TRACE("Format: " + FormatToString(format));
      ScopedGbmBo bo(gbm_bo_create(gbm_.get(), size, size, format, usage));
      EXPECT_BO(bo.get());
      found = true;
    }
    EXPECT_TRUE(found) << "Available format is not found";
  }
}

// Tests user data.
TEST_F(GraphicsGbmTest, UserData) {
  ScopedGbmBo bo1(gbm_bo_create(
      gbm_.get(), 1024, 1024, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
  ASSERT_BO(bo1.get());
  ScopedGbmBo bo2(gbm_bo_create(
      gbm_.get(), 1024, 1024, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
  ASSERT_BO(bo2.get());

  bool destroyed1 = false;
  bool destroyed2 = false;
  auto destroy = [](struct gbm_bo* bo, void* data) {
                   *static_cast<bool*>(data) = true;
                 };
  gbm_bo_set_user_data(bo1.get(), &destroyed1, destroy);
  gbm_bo_set_user_data(bo2.get(), &destroyed2, destroy);

  EXPECT_EQ(gbm_bo_get_user_data(bo1.get()), &destroyed1);
  EXPECT_EQ(gbm_bo_get_user_data(bo2.get()), &destroyed2);

  bo1.reset();
  EXPECT_TRUE(destroyed1);

  gbm_bo_set_user_data(bo2.get(), nullptr, nullptr);
  bo2.reset();
  EXPECT_FALSE(destroyed2);
}

// Tests prime export.
TEST_F(GraphicsGbmTest, Export) {
  ScopedGbmBo bo(gbm_bo_create(
      gbm_.get(), 1024, 1024, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
  ASSERT_BO(bo.get());

  base::ScopedFD prime_fd(gbm_bo_get_fd(bo.get()));
  EXPECT_TRUE(prime_fd.is_valid());
}

// Tests prime import using VGEM sharing buffer.
TEST_F(GraphicsGbmTest, ImportVgem) {
  constexpr uint32_t kWidth = 123;
  constexpr uint32_t kHeight = 456;

  base::ScopedFD vgem_fd = DrmOpenVgem();
  ASSERT_TRUE(vgem_fd.is_valid());
  struct drm_prime_handle prime_handle;
  ASSERT_EQ(0, CreateVgemBo(vgem_fd.get(), kWidth * kHeight * kBytesPerPixel,
                            &prime_handle.handle));
  prime_handle.flags = DRM_CLOEXEC;
  ASSERT_EQ(0, drmIoctl(vgem_fd.get(), DRM_IOCTL_PRIME_HANDLE_TO_FD,
                        &prime_handle));
  base::ScopedFD prime_fd(prime_handle.fd);

  struct gbm_import_fd_data fd_data;
  fd_data.fd = prime_fd.get();
  fd_data.width = kWidth;
  fd_data.height = kHeight;
  fd_data.stride = kWidth * kBytesPerPixel;
  fd_data.format = GBM_FORMAT_XRGB8888;

  ScopedGbmBo bo(gbm_bo_import(
      gbm_.get(), GBM_BO_IMPORT_FD, &fd_data, GBM_BO_USE_RENDERING));
  EXPECT_BO(bo.get());
}

// Tests prime import using dma-buf API.
TEST_F(GraphicsGbmTest, ImportDmabuf) {
  constexpr uint32_t kWidth = 123;
  constexpr uint32_t kHeight = 456;

  ScopedGbmBo bo1(gbm_bo_create(
      gbm_.get(), kWidth, kHeight, GBM_FORMAT_XRGB8888, GBM_BO_USE_RENDERING));
  ASSERT_BO(bo1.get());

  base::ScopedFD prime_fd(gbm_bo_get_fd(bo1.get()));
  ASSERT_TRUE(prime_fd.is_valid());

  struct gbm_import_fd_data fd_data;
  fd_data.fd = prime_fd.get();
  fd_data.width = kWidth;
  fd_data.height = kHeight;
  fd_data.stride = gbm_bo_get_stride(bo1.get());
  fd_data.format = GBM_FORMAT_XRGB8888;

  bo1.reset();

  ScopedGbmBo bo2(gbm_bo_import(
      gbm_.get(), GBM_BO_IMPORT_FD, &fd_data, GBM_BO_USE_RENDERING));
  ASSERT_BO(bo2.get());
  EXPECT_EQ(kWidth, gbm_bo_get_width(bo2.get()));
  EXPECT_EQ(kHeight, gbm_bo_get_height(bo2.get()));
  EXPECT_EQ(fd_data.stride, gbm_bo_get_stride(bo2.get()));
}

// Tests GBM_BO_IMPORT_FD_MODIFIER entry point.
TEST_F(GraphicsGbmTest, ImportModifier) {
  constexpr uint32_t kWidth = 567;
  constexpr uint32_t kHeight = 891;

  for (const auto format : kFormatList) {
    if (!gbm_device_is_format_supported(
            gbm_.get(), format, GBM_BO_USE_RENDERING)) {
      continue;
    }
    SCOPED_TRACE("Format: " + std::to_string(format));
    ScopedGbmBo bo1(gbm_bo_create(
        gbm_.get(), kWidth, kHeight, format, GBM_BO_USE_RENDERING));
    const size_t num_planes = gbm_bo_get_plane_count(bo1.get());

    std::vector<base::ScopedFD> fds;
    for (size_t p = 0; p < num_planes; ++p)
      fds.emplace_back(gbm_bo_get_plane_fd(bo1.get(), p));

    struct gbm_import_fd_modifier_data fd_data;
    fd_data.num_fds = num_planes;
    for (size_t p = 0; p < num_planes; ++p) {
      fd_data.fds[p] = fds[p].get();
      fd_data.strides[p] = gbm_bo_get_stride_for_plane(bo1.get(), p);
      fd_data.offsets[p] = gbm_bo_get_offset(bo1.get(), p);
    }
    fd_data.modifier = gbm_bo_get_modifier(bo1.get());
    fd_data.width = kWidth;
    fd_data.height = kHeight;
    fd_data.format = format;

    bo1.reset();

    ScopedGbmBo bo2(gbm_bo_import(
        gbm_.get(), GBM_BO_IMPORT_FD_MODIFIER, &fd_data, GBM_BO_USE_RENDERING));

    ASSERT_BO(bo2.get());
    EXPECT_EQ(kWidth, gbm_bo_get_width(bo2.get()));
    EXPECT_EQ(kHeight, gbm_bo_get_height(bo2.get()));
    EXPECT_EQ(fd_data.modifier, gbm_bo_get_modifier(bo2.get()));

    for (size_t p = 0; p < num_planes; ++p) {
      EXPECT_EQ(fd_data.strides[p], gbm_bo_get_stride_for_plane(bo2.get(), p))
          << "Unexpected stride at " << p;
      EXPECT_EQ(fd_data.offsets[p], gbm_bo_get_offset(bo2.get(), p))
          << "Unexpected offset at " << p;
    }
  }
}

TEST_F(GraphicsGbmTest, GemMap) {
  constexpr uint32_t kWidth = 666;
  constexpr uint32_t kHeight = 777;

  ScopedGbmBo bo(gbm_bo_create(
      gbm_.get(), kWidth, kHeight, GBM_FORMAT_ARGB8888,
      GBM_BO_USE_SW_READ_RARELY | GBM_BO_USE_SW_WRITE_RARELY));
  ASSERT_BO(bo.get());

  {
    void* raw_map_data = nullptr;
    uint32_t stride = 0;
    void* addr = gbm_bo_map(bo.get(), 0, 0, kWidth, kHeight,
                            GBM_BO_TRANSFER_READ_WRITE, &stride, &raw_map_data,
                            0);
    ASSERT_NE(MAP_FAILED, addr);
    ASSERT_TRUE(raw_map_data);
    ScopedGbmBoMap map_data(raw_map_data, bo.get());
    EXPECT_GT(stride, 0);

    uint32_t* pixel = static_cast<uint32_t*>(addr);
    pixel[(kHeight / 2) * (stride / kBytesPerPixel) + kWidth / 2] = 0xABBAABBA;
  }

  // Remap and verify written previously data.
  {
    void* raw_map_data = nullptr;
    uint32_t stride = 0;
    void* addr = gbm_bo_map(bo.get(), 0, 0, kWidth, kHeight,
                            GBM_BO_TRANSFER_READ_WRITE, &stride, &raw_map_data,
                            0);
    ASSERT_NE(MAP_FAILED, addr);
    ASSERT_TRUE(raw_map_data);
    ScopedGbmBoMap map_data(raw_map_data, bo.get());
    EXPECT_GT(stride, 0);
    uint32_t* pixel = static_cast<uint32_t*>(addr);
    EXPECT_EQ(0xABBAABBA,
              pixel[(kHeight / 2) * (stride / kBytesPerPixel) + kWidth / 2]);
  }
}

TEST_F(GraphicsGbmTest, DmabufMap) {
  constexpr uint32_t kWidth = 666;
  constexpr uint32_t kHeight = 777;

  ScopedGbmBo bo(gbm_bo_create(
      gbm_.get(), kWidth, kHeight, GBM_FORMAT_ARGB8888, GBM_BO_USE_LINEAR));
  ASSERT_BO(bo.get());

  {
    base::ScopedFD prime_fd(gbm_bo_get_fd(bo.get()));
    ASSERT_TRUE(prime_fd.is_valid());

    uint32_t stride = gbm_bo_get_stride(bo.get());
    ASSERT_GT(stride, 0);
    uint32_t length = gbm_bo_get_plane_size(bo.get(), 0);
    ASSERT_GT(length, 0);

    ScopedMmapMemory addr(
        mmap(nullptr, length, (PROT_READ | PROT_WRITE), MAP_SHARED,
             prime_fd.get(), 0),
        length);
    ASSERT_TRUE(addr.is_valid());

    uint32_t* pixel = static_cast<uint32_t*>(addr.get());
    size_t stride_pixels = stride / kBytesPerPixel;

    struct dma_buf_sync sync_start = {
      DMA_BUF_SYNC_START | DMA_BUF_SYNC_WRITE
    };
    ASSERT_EQ(
        0,
        HANDLE_EINTR(ioctl(prime_fd.get(), DMA_BUF_IOCTL_SYNC, &sync_start)));

    for (uint32_t y = 0; y < kHeight; ++y) {
      for (uint32_t x = 0; x < kWidth; ++x)
        pixel[y * stride_pixels + x] = ((y << 16) | x);
    }

    struct dma_buf_sync sync_end = { DMA_BUF_SYNC_END | DMA_BUF_SYNC_WRITE};
    ASSERT_EQ(
        0,
        HANDLE_EINTR(ioctl(prime_fd.get(), DMA_BUF_IOCTL_SYNC, &sync_end)));
  }

  {
    base::ScopedFD prime_fd(gbm_bo_get_fd(bo.get()));
    ASSERT_TRUE(prime_fd.is_valid());

    uint32_t stride = gbm_bo_get_stride(bo.get());
    ASSERT_GT(stride, 0);
    uint32_t length = gbm_bo_get_plane_size(bo.get(), 0);
    ASSERT_GT(length, 0);

    ScopedMmapMemory addr(
        mmap(nullptr, length, (PROT_READ | PROT_WRITE), MAP_SHARED,
             prime_fd.get(), 0),
        length);
    ASSERT_TRUE(addr.is_valid());

    uint32_t* pixel = static_cast<uint32_t*>(addr.get());
    size_t stride_pixels = stride / kBytesPerPixel;

    struct dma_buf_sync sync_start = {
      DMA_BUF_SYNC_START | DMA_BUF_SYNC_WRITE
    };
    ASSERT_EQ(
        0,
        HANDLE_EINTR(ioctl(prime_fd.get(), DMA_BUF_IOCTL_SYNC, &sync_start)));

    for (uint32_t y = 0; y < kHeight; ++y) {
      for (uint32_t x = 0; x < kWidth; ++x) {
        EXPECT_EQ(((y << 16) | x), pixel[y * stride_pixels + x])
            << "Pixel mismatch at (" << x << ", " << y << ")";
      }
    }

    struct dma_buf_sync sync_end = { DMA_BUF_SYNC_END | DMA_BUF_SYNC_WRITE };
    ASSERT_EQ(
        0,
        HANDLE_EINTR(ioctl(prime_fd.get(), DMA_BUF_IOCTL_SYNC, &sync_end)));
  }

  void* raw_map_data = nullptr;
  uint32_t stride = 0;
  void* addr = gbm_bo_map(bo.get(), 0, 0, kWidth, kHeight,
                          GBM_BO_TRANSFER_READ_WRITE, &stride, &raw_map_data,
                          0);
  ASSERT_NE(MAP_FAILED, addr);
  ASSERT_TRUE(raw_map_data);
  ScopedGbmBoMap map_data(raw_map_data, bo.get());
  EXPECT_GT(stride, 0);
  uint32_t* pixel = static_cast<uint32_t*>(addr);
  size_t stride_pixels = stride / kBytesPerPixel;

  for (uint32_t y = 0; y < kHeight; ++y) {
    for (uint32_t x = 0; x < kWidth; ++x) {
      EXPECT_EQ(((y << 16) | x), pixel[y * stride_pixels + x])
          << "Pixel mismatch at (" << x << ", " << y << ")";
    }
  }
}

TEST_F(GraphicsGbmTest, GemMapTiling) {
  // TODO(crbug.com/752669)
  if (strcmp(gbm_device_get_backend_name(gbm_.get()), "tegra"))
    return;

  constexpr uint32_t kWidth = 666;
  constexpr uint32_t kHeight = 777;

  for (const auto buffer_create_flag : kBufferList) {
    ScopedGbmBo bo(gbm_bo_create(
        gbm_.get(), kWidth, kHeight, GBM_FORMAT_ARGB8888, buffer_create_flag));
    ASSERT_BO(bo.get());

    {
      void* raw_map_data = nullptr;
      uint32_t stride = 0;
      void* addr = gbm_bo_map(bo.get(), 0, 0, kWidth, kHeight,
                              GBM_BO_TRANSFER_WRITE, &stride, &raw_map_data, 0);
      ASSERT_NE(MAP_FAILED, addr);
      ASSERT_TRUE(addr);
      ASSERT_TRUE(raw_map_data);
      ScopedGbmBoMap map_data(raw_map_data, bo.get());

      uint32_t* pixel = static_cast<uint32_t*>(addr);
      const uint32_t stride_pixels = stride / kBytesPerPixel;

      for (uint32_t y = 0; y < kHeight; ++y) {
        for (uint32_t x = 0; x < kWidth; ++x)
          pixel[y * stride_pixels + x] = ((y << 16 | x));
      }
    }

    // Remap and verify written previously data.
    {
      void* raw_map_data = nullptr;
      uint32_t stride = 0;
      void* addr = gbm_bo_map(bo.get(), 0, 0, kWidth, kHeight,
                              GBM_BO_TRANSFER_WRITE, &stride, &raw_map_data, 0);
      ASSERT_NE(MAP_FAILED, addr);
      ASSERT_TRUE(addr);
      ASSERT_TRUE(raw_map_data);
      ScopedGbmBoMap map_data(raw_map_data, bo.get());

      uint32_t* pixel = static_cast<uint32_t*>(addr);
      const uint32_t stride_pixels = stride / kBytesPerPixel;

      for (uint32_t y = 0; y < kHeight; ++y) {
        for (uint32_t x = 0; x < kWidth; ++x) {
          EXPECT_EQ(((y << 16) | x), pixel[y * stride_pixels + x])
              << "Pixel mismatch at (" << x << ", " << y << ")";
        }
      }
    }
  }
}

}  // namespace

int main(int argc, char** argv) {
  testing::InitGoogleTest(&argc, argv);
  return RUN_ALL_TESTS();
}
