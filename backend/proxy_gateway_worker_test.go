package backend

import (
	"testing"
	"time"
)

func TestProxyGatewayWorkerFindsOnlyExpiredUnattachedProfiles(t *testing.T) {
	now := time.Now()
	worker := &proxyGatewayWorker{profiles: map[string]*proxyGatewayWorkerProfile{
		"expired":  {preparedAt: now.Add(-3 * time.Minute)},
		"fresh":    {preparedAt: now.Add(-time.Minute)},
		"attached": {preparedAt: now.Add(-3 * time.Minute), browserPID: 1234},
		"legacy":   {},
	}}

	expired := worker.detachExpiredUnattachedProfilesLocked(now, 2*time.Minute)
	if len(expired) != 1 {
		t.Fatalf("detached profiles = %d, want 1", len(expired))
	}
	if worker.profiles["expired"] != nil {
		t.Fatal("expired unattached profile was not removed")
	}
	for _, profileID := range []string{"fresh", "attached", "legacy"} {
		if worker.profiles[profileID] == nil {
			t.Fatalf("profile %q was removed unexpectedly", profileID)
		}
	}
}
