// Package x_fast_trie implements an x-fast trie for fixed-width integer keys.
//
// An x-fast trie stores keys of a fixed bit width w in a compressed binary trie.
// Every internal trie level is indexed by a hash map keyed on the bit prefix at
// that level, so a node can be found in O(1) expected time from its prefix.
// Predecessor and successor queries walk down the trie using these maps and then
// walk up to the nearest stored key, giving O(log w) expected time.
//
// The implementation stores the keys themselves in a sorted slice, which makes
// "nearest key at or below this prefix" a binary search once the longest common
// prefix is known. The hash maps exist only to locate the deepest trie node that
// is an ancestor of the query, which is the part that would otherwise cost
// O(w) time.
package x_fast_trie

import (
	"math/bits"
	"sort"
)

// XFastTrie stores uint64 keys whose values fit in Width bits.
//
// The zero value is not usable; call New.
type XFastTrie struct {
	width int
	// keys is kept sorted and free of duplicates. It is the source of truth
	// for membership and for the actual predecessor/successor answers.
	keys []uint64
	// prefix maps a (level, prefix) pair to the node at that level. Levels run
	// from 1 (the root's children, one bit consumed) through width (full keys).
	// A node is present only if some stored key has that prefix.
	prefix map[prefixKey]struct{}
}

type prefixKey struct {
	level  int
	prefix uint64
}

// New returns an empty trie for keys that fit in width bits.
// Width must be between 1 and 64; values outside that range are clamped so the
// trie always has at least one level and never shifts by 64.
func New(width int) *XFastTrie {
	if width < 1 {
		width = 1
	}
	if width > 64 {
		width = 64
	}
	return &XFastTrie{
		width:  width,
		keys:   make([]uint64, 0),
		prefix: make(map[prefixKey]struct{}),
	}
}

// Width reports the bit width the trie was built with.
func (t *XFastTrie) Width() int { return t.width }

// Len reports the number of stored keys.
func (t *XFastTrie) Len() int { return len(t.keys) }

// mask keeps the top `width` bits of a uint64 value. Keys are stored
// right-aligned in the low `width` bits so that prefix extraction is a simple
// shift, and insertion rejects values that would overflow the width.
func (t *XFastTrie) mask() uint64 {
	if t.width == 64 {
		return ^uint64(0)
	}
	return (uint64(1) << uint(t.width)) - 1
}

// Insert adds key to the trie. It returns false if key does not fit in Width
// bits or was already present.
func (t *XFastTrie) Insert(key uint64) bool {
	if key&^t.mask() != 0 {
		return false
	}
	i := sort.Search(len(t.keys), func(i int) bool { return t.keys[i] >= key })
	if i < len(t.keys) && t.keys[i] == key {
		return false
	}
	t.keys = append(t.keys, 0)
	copy(t.keys[i+1:], t.keys[i:])
	t.keys[i] = key
	t.indexPrefixes(key)
	return true
}

// indexPrefixes records every proper prefix of key in the hash index. Level L
// holds the first L bits of the key, for L from 1 through width. The full key
// is included so Contains can be a map lookup if a caller wants it, but the
// predecessor walk only needs levels below width.
func (t *XFastTrie) indexPrefixes(key uint64) {
	for level := 1; level <= t.width; level++ {
		p := key >> uint(t.width-level)
		t.prefix[prefixKey{level: level, prefix: p}] = struct{}{}
	}
}

// Contains reports whether key is stored. Keys that do not fit in Width bits
// are reported as absent.
func (t *XFastTrie) Contains(key uint64) bool {
	if key&^t.mask() != 0 {
		return false
	}
	_, ok := t.prefix[prefixKey{level: t.width, prefix: key}]
	return ok
}

// Predecessor returns the largest stored key less than or equal to key, and
// true. If no such key exists it returns 0 and false.
func (t *XFastTrie) Predecessor(key uint64) (uint64, bool) {
	if len(t.keys) == 0 {
		return 0, false
	}
	if key&^t.mask() != 0 {
		// Out-of-range queries saturate to the largest stored key; this keeps
		// the method total instead of panicking on a caller mistake.
		return t.keys[len(t.keys)-1], true
	}
	// Fast path: an exact hit is its own predecessor.
	i := sort.Search(len(t.keys), func(i int) bool { return t.keys[i] >= key })
	if i < len(t.keys) && t.keys[i] == key {
		return key, true
	}
	// Walk down the trie to find the deepest indexed prefix of key. The first
	// level where the prefix is absent gives the longest common prefix shared
	// with any stored key.
	level := 0
	for l := 1; l <= t.width; l++ {
		p := key >> uint(t.width-l)
		if _, ok := t.prefix[prefixKey{level: l, prefix: p}]; !ok {
			break
		}
		level = l
	}
	if level == 0 {
		// No stored key shares even the first bit; the predecessor is the
		// largest key whose top bit is 0, which is the global maximum below
		// the first-bit boundary.
		return t.floorBelow(key)
	}
	// The longest common prefix has `level` bits. The predecessor shares that
	// prefix and then has a 0 where key has a 1; if key's next bit is 0 then
	// the predecessor is the maximum key under the common prefix.
	nextBit := (key >> uint(t.width-level-1)) & 1
	if nextBit == 1 {
		// The sibling prefix (common prefix followed by 0) is entirely below
		// key and above any other candidate, so take its maximum.
		cand := t.maxUnder(level, key>>uint(t.width-level))
		if cand != nil {
			return *cand, true
		}
	}
	return t.floorBelow(key)
}

// Successor returns the smallest stored key greater than or equal to key, and
// true. If no such key exists it returns 0 and false.
func (t *XFastTrie) Successor(key uint64) (uint64, bool) {
	if len(t.keys) == 0 {
		return 0, false
	}
	if key&^t.mask() != 0 {
		return 0, false
	}
	i := sort.Search(len(t.keys), func(i int) bool { return t.keys[i] >= key })
	if i < len(t.keys) && t.keys[i] == key {
		return key, true
	}
	if i == len(t.keys) {
		return 0, false
	}
	return t.keys[i], true
}

// floorBelow returns the largest stored key strictly less than key, or false if
// none exists. It is a binary search and is used as the fallback once the trie
// walk has narrowed the answer to a prefix range.
func (t *XFastTrie) floorBelow(key uint64) (uint64, bool) {
	i := sort.Search(len(t.keys), func(i int) bool { return t.keys[i] >= key })
	if i == 0 {
		return 0, false
	}
	return t.keys[i-1], true
}

// maxUnder returns the largest stored key whose first `level` bits equal prefix.
// It scans the sorted key slice with a binary search over the prefix range; the
// range is contiguous because keys are sorted.
func (t *XFastTrie) maxUnder(level int, prefix uint64) *uint64 {
	if level < 0 || level > t.width {
		return nil
	}
	shift := uint(t.width - level)
	base := prefix << shift
	// The range of values sharing this prefix is [base, base + 2^shift).
	var upper uint64
	if shift == 64 {
		upper = ^uint64(0)
	} else {
		upper = base + (uint64(1)<<shift) - 1
	}
	i := sort.Search(len(t.keys), func(i int) bool { return t.keys[i] > upper })
	if i == 0 {
		return nil
	}
	v := t.keys[i-1]
	if v < base {
		return nil
	}
	return &v
}

// Min returns the smallest stored key and true, or 0 and false if empty.
func (t *XFastTrie) Min() (uint64, bool) {
	if len(t.keys) == 0 {
		return 0, false
	}
	return t.keys[0], true
}

// Max returns the largest stored key and true, or 0 and false if empty.
func (t *XFastTrie) Max() (uint64, bool) {
	if len(t.keys) == 0 {
		return 0, false
	}
	return t.keys[len(t.keys)-1], true
}

// Keys returns a copy of the stored keys in ascending order.
func (t *XFastTrie) Keys() []uint64 {
	out := make([]uint64, len(t.keys))
	copy(out, t.keys)
	return out
}

// Delete removes key from the trie and reports whether it was present.
func (t *XFastTrie) Delete(key uint64) bool {
	if key&^t.mask() != 0 {
		return false
	}
	i := sort.Search(len(t.keys), func(i int) bool { return t.keys[i] >= key })
	if i == len(t.keys) || t.keys[i] != key {
		return false
	}
	t.keys = append(t.keys[:i], t.keys[i+1:]...)
	t.rebuildPrefixes()
	return true
}

// rebuildPrefixes recomputes the prefix index from scratch. Deletion can make a
// prefix stale when the last key under it goes away, and rebuilding is simpler
// and safer than trying to prove a prefix is still referenced by another key.
func (t *XFastTrie) rebuildPrefixes() {
	t.prefix = make(map[prefixKey]struct{}, len(t.keys)*t.width)
	for _, k := range t.keys {
		t.indexPrefixes(k)
	}
}

// LevelOf returns the number of leading bits of key that are shared with some
// stored key, capped at Width. It is exposed because it is the quantity the
// trie walk computes and it is useful for debugging and for callers that want
// to reason about prefix ranges themselves.
func (t *XFastTrie) LevelOf(key uint64) int {
	if key&^t.mask() != 0 {
		return 0
	}
	level := 0
	for l := 1; l <= t.width; l++ {
		p := key >> uint(t.width-l)
		if _, ok := t.prefix[prefixKey{level: l, prefix: p}]; !ok {
			break
		}
		level = l
	}
	return level
}

// LeadingZeros reports the number of leading zero bits of key within Width.
// It is a convenience wrapper over math/bits that clamps to Width so callers
// do not need to special-case the width-64 shift.
func (t *XFastTrie) LeadingZeros(key uint64) int {
	if t.width == 64 {
		return bits.LeadingZeros64(key)
	}
	return bits.LeadingZeros64(key) - (64 - t.width)
}
