/**
 * Keyboard arithmetic of the page, kept free of the DOM so it runs under
 * `node --test`.
 */

/**
 * The index after moving the selection by delta positions in a list of
 * count items, wrapping around at both ends. The double modulo keeps the
 * result in range even when the jump is larger than the list, which happens
 * on the apps grid when a search leaves fewer results than columns.
 * @param {number} selected
 * @param {number} delta
 * @param {number} count
 * @returns {number}
 */
export function nextIndex(selected, delta, count) {
    if (count === 0) {
        return 0;
    }

    return (((selected + delta) % count) + count) % count;
}
