# x_fast_trie

An x-fast trie over fixed-width integer keys: it stores `uint64` values that fit
in a chosen number of bits and answers predecessor and successor queries in
O(log w) expected time using hash-indexed prefix nodes.

```go
package main

import (
	"fmt"

	xfast "x_fast_trie"
)

func main() {
	tr := xfast.New(16)
	tr.Insert(0x0100)
	tr.Insert(0x0200)
	tr.Insert(0x0300)

	if p, ok := tr.Predecessor(0x0250); ok {
		fmt.Printf("predecessor: %#x\n", p) // 0x200
	}
	if s, ok := tr.Successor(0x0250); ok {
		fmt.Printf("successor: %#x\n", s) // 0x300
	}
	fmt.Println(tr.Contains(0x0200)) // true
}
```

The exported API is:

- `New(width int) *XFastTrie`
- `(*XFastTrie) Insert(key uint64) bool`
- `(*XFastTrie) Delete(key uint64) bool`
- `(*XFastTrie) Contains(key uint64) bool`
- `(*XFastTrie) Predecessor(key uint64) (uint64, bool)`
- `(*XFastTrie) Successor(key uint64) (uint64, bool)`
- `(*XFastTrie) Min() (uint64, bool)`
- `(*XFastTrie) Max() (uint64, bool)`
- `(*XFastTrie) Keys() []uint64`
- `(*XFastTrie) Len() int`
- `(*XFastTrie) Width() int`
- `(*XFastTrie) LevelOf(key uint64) int`
- `(*XFastTrie) LeadingZeros(key uint64) int`

## Why this exists

A plain binary trie gives O(w) predecessor queries because you walk one bit at a
time. The x-fast trie keeps a hash map for every trie level, keyed by the bit
prefix at that level, so the walk down to the deepest shared prefix can be done
by probing maps instead of following pointers. That is the whole point of the
structure, and it is what this package implements.

The trade-off is memory: the maps hold one entry per key per level, so the trie
uses O(n log w) space rather than the O(n) of a balanced search tree. In return,
`Predecessor` and `Successor` are expected O(log w) after the initial descent.
The actual keys live in a sorted slice, which keeps `Min`, `Max`, `Keys`, and
`Delete` simple and correct; the maps only exist to locate prefixes quickly.

## The awkward edge

Keys are right-aligned in their low `width` bits, and values that do not fit in
`width` bits are rejected by `Insert` and reported absent by `Contains` and
`Successor`. `Predecessor` is the exception: an out-of-range query saturates to
the largest stored key instead of returning false, so that a caller who probes
past the top of the key space still gets a meaningful answer. If you need strict
range checking, test the query against `Width` before calling.
