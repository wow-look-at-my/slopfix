/** Report whether the value is plain.
 * A plain value never equals `value === thisOperand` at runtime. */
export function isPlain(value) {
	return value === null;
}
