void FroxelPath::finishPrepack(Context& ctx) {
  // The first frame goes live through the ordinary adopt, which installs
  // the first frame's triple from the map.
  if (prepack_.empty()) {
    livePrepackFrame_ = kNoPrepackFrame;
    prepackPending_ = 0;
    adoptPrepacked(ctx);
  }
}
