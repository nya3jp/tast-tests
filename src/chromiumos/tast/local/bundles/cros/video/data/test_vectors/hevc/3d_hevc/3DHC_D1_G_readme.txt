Name: 3DHC_DT_G
Specification: The bitstream is coded with random access configuration as well as 3-view configuration. In addition to inter-view sample prediction for depth, the all depth Intra tools are enabled together with depth lookup table (DLT).
Functional stage: Decoding of three views with depth Intra coding tools enabled.
Purpose: To conform the depth Intra coding tools in random access configuration.
frame number:64
Software: HTM15.1
Configure(base): baseCfg_3view+depth.cfg + qpCfg_Nview+depth_QP40.cfg + seqCfg_Kendo.cfg
  Configure: --IvMvPredFlag 1 0 --IvResPredFlag 0 --IlluCompEnable 0 --IlluCompLowLatencyEnc 0 --ViewSynthesisPredFlag 0 --DepthRefinementFlag 0 --IvMvScalingFlag 0 --Log2SubPbSizeMinus3 3 --Log2MpiSubPbSizeMinus3 3 --DepthBasedBlkPartFlag 0 --VSO 1 --IntraWedgeFlag 1 --IntraContourFlag 0 --IntraSdcFlag 1 --DLT 1 --QTL 0 --QtPredFlag 0 --InterSdcFlag 0 --MpiFlag 0 --DepthIntraSkip 1