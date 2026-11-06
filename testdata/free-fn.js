#[no_mangle]
pub unsafe extern "C" fn converter_free(handle: *mut Handle) {
    if handle.is_null() {
        return;
    }
    let owned = Box::from_raw(handle);
    let streams = owned.streams.lock().unwrap();
    for (index, stream) in streams.iter() {
        stream.flush(*index);
        stream.close(*index);
    }
    drop(streams);
    owned.tokenizer.release();
    owned.parser.release();
    owned.decoder.release();
    owned.close();
    drop(owned);
}
