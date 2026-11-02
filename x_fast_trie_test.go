package x_fast_trie

import (
	"sort"
	"testing"
)

func TestInsertAndContains(t *testing.T) {
	tr := New(8)
	if !tr.Insert(0x12) {
		t.Fatal("Insert(0x12) = false, want true")
	}
	if !tr.Insert(0x34) {
		t.Fatal("Insert(0x34) = false, want true")
	}
	if tr.Insert(0x12) {
		t.Fatal("duplicate Insert returned true")
	}
	if tr.Insert(0x100) {
		t.Fatal("Insert of out-of-range key returned true")
	}
	if !tr.Contains(0x12) {
		t.Fatal("Contains(0x12) = false, want true")
	}
	if tr.Contains(0x13) {
		t.Fatal("Contains(0x13) = true, want false")
	}
	if tr.Contains(0x100) {
		t.Fatal("Contains of out-of-range key = true, want false")
	}
	if got := tr.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
}

func TestPredecessorExactAndBetween(t *testing.T) {
	tr := New(8)
	for _, k := range []uint64{0x10, 0x20, 0x30, 0x40} {
		tr.Insert(k)
	}
	cases := []struct {
		key  uint64
		want uint64
		ok   bool
	}{
		{0x10, 0x10, true},
		{0x18, 0x10, true},
		{0x20, 0x20, true},
		{0x2f, 0x20, true},
		{0x40, 0x40, true},
		{0x41, 0x40, true},
		{0x0f, 0, false},
	}
	for _, c := range cases {
		got, ok := tr.Predecessor(c.key)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("Predecessor(%#x) = (%#x, %v), want (%#x, %v)", c.key, got, ok, c.want, c.ok)
		}
	}
}

func TestSuccessor(t *testing.T) {
	tr := New(8)
	for _, k := range []uint64{0x10, 0x20, 0x30} {
		tr.Insert(k)
	}
	cases := []struct {
		key  uint64
		want uint64
		ok   bool
	}{
		{0x10, 0x10, true},
		{0x11, 0x20, true},
		{0x1f, 0x20, true},
		{0x30, 0x30, true},
		{0x31, 0, false},
	}
	for _, c := range cases {
		got, ok := tr.Successor(c.key)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("Successor(%#x) = (%#x, %v), want (%#x, %v)", c.key, got, ok, c.want, c.ok)
		}
	}
}

func TestPredecessorWithSharedPrefix(t *testing.T) {
	// All keys share the top 4 bits, so the trie walk must descend several
	// levels before the prefix index misses.
	tr := New(8)
	for _, k := range []uint64{0xf0, 0xf1, 0xf2, 0xf3} {
		tr.Insert(k)
	}
	got, ok := tr.Predecessor(0xf2)
	if !ok || got != 0xf2 {
		t.Fatalf("Predecessor(0xf2) = (%#x, %v), want (0xf2, true)", got, ok)
	}
	got, ok = tr.Predecessor(0xf4)
	if !ok || got != 0xf3 {
		t.Fatalf("Predecessor(0xf4) = (%#x, %v), want (0xf3, true)", got, ok)
	}
	got, ok = tr.Predecessor(0xef)
	if ok {
		t.Fatalf("Predecessor(0xef) = (%#x, %v), want (_, false)", got, ok)
	}
}

func TestPredecessorAcrossTopBitBoundary(t *testing.T) {
	tr := New(8)
	tr.Insert(0x00)
	tr.Insert(0x80)
	got, ok := tr.Predecessor(0x7f)
	if !ok || got != 0x00 {
		t.Fatalf("Predecessor(0x7f) = (%#x, %v), want (0x00, true)", got, ok)
	}
	got, ok = tr.Predecessor(0x80)
	if !ok || got != 0x80 {
		t.Fatalf("Predecessor(0x80) = (%#x, %v), want (0x80, true)", got, ok)
	}
	got, ok = tr.Predecessor(0xff)
	if !ok || got != 0x80 {
		t.Fatalf("Predecessor(0xff) = (%#x, %v), want (0x80, true)", got, ok)
	}
}

func TestEmptyTrie(t *testing.T) {
	tr := New(16)
	if _, ok := tr.Predecessor(0); ok {
		t.Fatal("Predecessor on empty trie returned ok")
	}
	if _, ok := tr.Successor(0); ok {
		t.Fatal("Successor on empty trie returned ok")
	}
	if _, ok := tr.Min(); ok {
		t.Fatal("Min on empty trie returned ok")
	}
	if _, ok := tr.Max(); ok {
		t.Fatal("Max on empty trie returned ok")
	}
	if got := tr.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
}

func TestDeleteRebuildsPrefixes(t *testing.T) {
	tr := New(8)
	tr.Insert(0x10)
	tr.Insert(0x11)
	if !tr.Delete(0x10) {
		t.Fatal("Delete(0x10) = false, want true")
	}
	if tr.Delete(0x10) {
		t.Fatal("second Delete(0x10) = true, want false")
	}
	if tr.Contains(0x10) {
		t.Fatal("Contains(0x10) = true after delete")
	}
	if !tr.Contains(0x11) {
		t.Fatal("Contains(0x11) = false after deleting sibling")
	}
	got, ok := tr.Predecessor(0x10)
	if ok {
		t.Fatalf("Predecessor(0x10) = (%#x, %v), want (_, false)", got, ok)
	}
	got, ok = tr.Predecessor(0x11)
	if !ok || got != 0x11 {
		t.Fatalf("Predecessor(0x11) = (%#x, %v), want (0x11, true)", got, ok)
	}
}

func TestWidthClampingAndOutOfRange(t *testing.T) {
	wide := New(0)
	if wide.Width() != 1 {
		t.Fatalf("New(0).Width() = %d, want 1", wide.Width())
	}
	if !wide.Insert(1) {
		t.Fatal("Insert(1) into width-1 trie = false, want true")
	}
	if wide.Insert(2) {
		t.Fatal("Insert(2) into width-1 trie = true, want false")
	}

	tr := New(64)
	if tr.Width() != 64 {
		t.Fatalf("New(64).Width() = %d, want 64", tr.Width())
	}
	if !tr.Insert(^uint64(0)) {
		t.Fatal("Insert(max uint64) into width-64 trie = false, want true")
	}
	if !tr.Contains(^uint64(0)) {
		t.Fatal("Contains(max uint64) = false, want true")
	}
}

func TestKeysAreSortedCopy(t *testing.T) {
	tr := New(8)
	for _, k := range []uint64{0x30, 0x10, 0x20} {
		tr.Insert(k)
	}
	got := tr.Keys()
	want := []uint64{0x10, 0x20, 0x30}
	if len(got) != len(want) {
		t.Fatalf("Keys() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Keys()[%d] = %#x, want %#x", i, got[i], want[i])
		}
	}
	got[0] = 0xff
	again := tr.Keys()
	if again[0] != 0x10 {
		t.Fatal("Keys() returned the internal slice")
	}
}

func TestPredecessorMatchesLinearScan(t *testing.T) {
	// Deterministic pseudo-random keys; no time or randomness involved.
	tr := New(12)
	var keys []uint64
	x := uint64(1)
	for i := 0; i < 200; i++ {
		x = x*1103515245 + 12345
		k := x & 0xfff
		if tr.Insert(k) {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for q := uint64(0); q <= 0xfff; q += 7 {
		want, ok := linearPredecessor(keys, q)
		got, gotOK := tr.Predecessor(q)
		if ok != gotOK || (ok && got != want) {
			t.Fatalf("Predecessor(%#x) = (%#x, %v), want (%#x, %v)", q, got, gotOK, want, ok)
		}
	}
}

func TestSuccessorMatchesLinearScan(t *testing.T) {
	tr := New(12)
	var keys []uint64
	x := uint64(7)
	for i := 0; i < 200; i++ {
		x = x*1103515245 + 12345
		k := x & 0xfff
		if tr.Insert(k) {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for q := uint64(0); q <= 0xfff; q += 7 {
		want, ok := linearSuccessor(keys, q)
		got, gotOK := tr.Successor(q)
		if ok != gotOK || (ok && got != want) {
			t.Fatalf("Successor(%#x) = (%#x, %v), want (%#x, %v)", q, got, gotOK, want, ok)
		}
	}
}

func TestLevelOf(t *testing.T) {
	tr := New(8)
	tr.Insert(0xf0)
	if got := tr.LevelOf(0xf0); got != 8 {
		t.Fatalf("LevelOf(0xf0) = %d, want 8", got)
	}
	if got := tr.LevelOf(0xf1); got != 7 {
		t.Fatalf("LevelOf(0xf1) = %d, want 7", got)
	}
	if got := tr.LevelOf(0x00); got != 0 {
		t.Fatalf("LevelOf(0x00) = %d, want 0", got)
	}
}

func TestMinMax(t *testing.T) {
	tr := New(16)
	tr.Insert(0xbeef)
	tr.Insert(0x0001)
	tr.Insert(0x8000)
	if got, ok := tr.Min(); !ok || got != 0x0001 {
		t.Fatalf("Min() = (%#x, %v), want (0x0001, true)", got, ok)
	}
	if got, ok := tr.Max(); !ok || got != 0xbeef {
		t.Fatalf("Max() = (%#x, %v), want (0xbeef, true)", got, ok)
	}
}

func linearPredecessor(keys []uint64, q uint64) (uint64, bool) {
	var best uint64
	found := false
	for _, k := range keys {
		if k <= q && (!found || k > best) {
			best = k
			found = true
		}
	}
	return best, found
}

func linearSuccessor(keys []uint64, q uint64) (uint64, bool) {
	var best uint64
	found := false
	for _, k := range keys {
		if k >= q && (!found || k < best) {
			best = k
			found = true
		}
	}
	return best, found
}
