3DHC_D2_B_LGE
Specification: The bitstream is coded with random access configuration as well as 3-view configuration. In addition to inter-view sample prediction for depth, the inter SDC is enabled for depth coding.
Functional stage: Decoding of three views with inter SDC enabled for depth views.
Purpose: To conform the inter SDC for depth.

Software version : HTM-15.1
Number of coded frames : 50
Coded Sequence : Shark
Frame rate : 30 fps
GOP size : 8
Intra period : 24
Level : 5.1
All Multiview coding tools are disabled, as well as depth coding tools except for VSO, DLT and InterSDC.

<HTM configuration>
#======== VPS / PTLI ================
Profile                       : main main 3d-main          # Profile indication in VpsProfileTierLevel, per VpsProfileTierLevel syntax structure  (m)
Level                         : 5.1 5.1 5.1                # Level   indication in VpsProfileTierLevel, per VpsProfileTierLevel syntax structure  (m)
Tier                          : main main main             # Tier    indication in VpsProfileTierLevel, per VpsProfileTierLevel syntax structure  (m)

#========== multiview coding tools ==========
IvMvPredFlag                        : 0 0              # Inter-view motion prediction
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
VSO                                 : 1                 # use of view synthesis optimization for depth coding
IntraWedgeFlag                      : 0
IntraContourFlag                    : 0                 # use of intra-view prediction mode
IntraSdcFlag                        : 0
DLT                                 : 1
QTL                                 : 0
QtPredFlag                          : 0
InterSdcFlag                        : 1                 # use of inter sdc
MpiFlag                             : 0
IntraSingleFlag                     : 0                 # use of single depth mode

#========== view synthesis optimization (VSO) ==========
VSOConfig                 : [cx0 B(cc1) I(s0.25 s0.5 s0.75)][cx1 B(oo0) B(oo2) I(s0.25 s0.5 s0.75 s1.25 s1.5 s1.75)][cx2 B(cc1) I(s1.25 s1.5 s1.75)] # VSO configuration string
WVSO                      : 1
VSOWeight                 : 10
VSDWeight                 : 1
DWeight                   : 1
UseEstimatedVSD           : 1