struct WarpPass {
  // The pass ships 4 hooks.
  //
  // It reads N px/texel (N in {1,2,4,8}; 0 = off).
  // Pin --warp-field 0 for the pre-field warp.
  void record(int n) {
    density_ = n;
    mask_ = 0;
    field_ = n * 2;
    cells_ = n + 1;
  }
};
