package identity

import "testing"

// fakeStore 是 UniquenessStore 的内存实现,用于测试重采逻辑。
type fakeStore struct {
	hashes map[string]bool
	seeds  map[int64]bool
}

func (f *fakeStore) Seen(fingerprintHash string, seed int64) (bool, error) {
	return f.hashes[fingerprintHash] || f.seeds[seed], nil
}

// 无冲突时,GenerateUnique 应直接返回首个生成的身份。
func TestGenerateUniqueReturnsFirstWhenNoCollision(t *testing.T) {
	store := &fakeStore{hashes: map[string]bool{}, seeds: map[int64]bool{}}
	id := sampleIdentity()
	got, err := GenerateUnique(store, func() Identity { return id }, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FingerprintHash() != id.FingerprintHash() {
		t.Fatal("expected the generated identity to be returned unchanged")
	}
}

// 首个候选与已登记项冲突时,应继续重采直到得到唯一身份。
func TestGenerateUniqueRetriesPastCollisions(t *testing.T) {
	a := sampleIdentity()
	b := sampleIdentity()
	b.Seed = a.Seed + 100 // 唯一
	store := &fakeStore{
		hashes: map[string]bool{a.FingerprintHash(): true},
		seeds:  map[int64]bool{a.Seed: true},
	}
	seq := []Identity{a, b}
	i := 0
	gen := func() Identity {
		id := seq[i]
		i++
		return id
	}
	got, err := GenerateUnique(store, gen, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Seed != b.Seed {
		t.Fatalf("expected the unique identity (seed %d), got seed %d", b.Seed, got.Seed)
	}
}

// 若始终撞车,超过最大尝试次数应返回错误(不静默返回重复身份)。
func TestGenerateUniqueErrorsWhenExhausted(t *testing.T) {
	a := sampleIdentity()
	store := &fakeStore{
		hashes: map[string]bool{a.FingerprintHash(): true},
		seeds:  map[int64]bool{a.Seed: true},
	}
	if _, err := GenerateUnique(store, func() Identity { return a }, 3); err == nil {
		t.Fatal("expected an error when all attempts collide")
	}
}
