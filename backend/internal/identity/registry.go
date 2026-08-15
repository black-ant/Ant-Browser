package identity

import "fmt"

// UniquenessStore 判定某套身份的唯一性键(指纹 hash / seed)是否已被占用。
// 由 DB 支持的实现负责查询 browser_identities 表。
type UniquenessStore interface {
	Seen(fingerprintHash string, seed int64) (bool, error)
}

// GenerateUnique 反复调用 gen 生成候选身份,直到其 fingerprint hash 与 seed 都未被
// store 占用为止;超过 maxAttempts 仍撞车则返回错误,绝不静默返回重复身份。
//
// 偏向真实分布:gen 应从真机指纹池加权采样,重采只是在极少数碰撞时兜底,
// 而非追求“最大唯一”。
func GenerateUnique(store UniquenessStore, gen func() Identity, maxAttempts int) (Identity, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		cand := gen()
		seen, err := store.Seen(cand.FingerprintHash(), cand.Seed)
		if err != nil {
			return Identity{}, err
		}
		if !seen {
			return cand, nil
		}
	}
	return Identity{}, fmt.Errorf("identity: 连续 %d 次采样均与已有环境冲突,无法生成唯一指纹", maxAttempts)
}
