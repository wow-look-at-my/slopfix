void FroxelPath::finishPrepack(Context& ctx) {
  ensureSplatCapacity(ctx, peakC);
  // The first frame goes live through the ordinary adopt. cubeCount_ = 0 forces the
  // adopt to install the triple of that frame from the map.
  livePrepackFrame_ = kNoPrepackFrame;
  prepackPending_ = 0;
  cubeCount_ = 0;
  adoptPrepacked(ctx);
}
