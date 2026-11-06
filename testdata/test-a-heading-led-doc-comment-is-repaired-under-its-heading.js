/// # Safety
///
/// The caller keeps the pointer valid for the whole call. No other thread writes through it. The buffer holds at least n bytes. The length counts bytes and not characters. A null pointer is never passed here.
unsafe fn first(p: *const u8, n: usize) -> u8 {
    let s = std::slice::from_raw_parts(p, n);
    let v = s[0];
    v
}
