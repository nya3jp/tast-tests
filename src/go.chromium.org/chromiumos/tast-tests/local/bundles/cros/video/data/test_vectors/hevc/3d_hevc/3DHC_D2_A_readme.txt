- Purpose: To conform the inter-view motion prediction for depth
- Bitstream file: 3DHC_D2_A.hevc
- MD5 file: 3DHC_D2_A.md5
- Test configuration
  . Software version: HTM 15.1
  . Test Sequence: Shark (5, 1, 9)
  . Resolution: 1920x1088
  . Frame rate: 30 fps
  . Number of frames: 60
  . View configuration: 3T + 3D (P-I-P)
    ¡æ Each view was coded with random access configuration
  . Following tools are turned off
    ¡æ inter-view motion prediction for texture
    ¡æ residual prediction
    ¡æ illumination compensation
    ¡æ view synthesis prediction
    ¡æ depth based disparity refinment
    ¡æ inter-view motion vector scaling
    ¡æ depth-based block partitioning
    ¡æ depth modeling mode
    ¡æ intra-view prediction mode
    ¡æ segment-wise DC coding for both intra and inter
    ¡æ depth look-up table
    ¡æ quad-tree limitation depth
    ¡æ depth intra skip
    ¡æ motion parameter inheritance
    <HTM configuration>
    #========== multiview coding tools ==========
    IvMvPredFlag                        : 0 1              # Inter-view motion prediction
    IvResPredFlag                       : 0                # Advanced inter-view residual prediction (0:off, 1:on)
    IlluCompEnable                      : 0                # Enable Illumination compensation ( 0: off, 1: on )  (v/d)
    IlluCompLowLatencyEnc               : 0                # Enable low-latency Illumination compensation encoding( 0: off, 1: on )
    ViewSynthesisPredFlag               : 0                # View synthesis prediction
    DepthRefinementFlag                 : 0                # Disparity refined by depth DoNBDV
    IvMvScalingFlag                     : 0                # Interview motion vector scaling
    Log2SubPbSizeMinus3                 : 0                # Log2 of sub-PU size minus 3 for IvMvPred (0 ... 3) and smaller than or equal to log2(maxCUSize)-3
    Log2MpiSubPbSizeMinus3              : 0                # Log2 of sub-PU size minus 3 for MPI (0 ... 3) and smaller than or equal to log2(maxCUSize)-3
    DepthBasedBlkPartFlag               : 0                # Depth-based Block Partitioning
    #========== depth coding tools ==========
    VSO                                 : 1                # use of view synthesis optimization for depth coding
    IntraWedgeFlag                      : 0
    IntraContourFlag                    : 0                # use of intra-view prediction mode
    IntraSdcFlag                        : 0
    DLT                                 : 0
    QTL                                 : 0
    QtPredFlag                          : 0
    InterSdcFlag                        : 0                 # use of inter sdc
    MpiFlag                             : 0
    DepthIntraSkip                      : 0                 # use of single depth mode
    #======== VPS / PTLI ================
    Profile                       : main main 3d-main       # Profile indication in VpsProfileTierLevel, per VpsProfileTierLevel syntax structure  (m)
    Level                         : 5.1 5.1 5.1             # Level   indication in VpsProfileTierLevel, per VpsProfileTierLevel syntax structure  (m)
    Tier                          : main main main          # Tier    indication in VpsProfileTierLevel, per VpsProfileTierLevel syntax structure  (m)
