package gateway

import "testing"

func TestResolveAction(t *testing.T) {
	config := RoutingConfig{
		Mode: ModeRule,
		Rules: []Rule{
			{Enabled: true, MatchType: MatchDomain, Pattern: "exact.example", Action: ActionBlock},
			{Enabled: true, MatchType: MatchDomainSuffix, Pattern: "example.com", Action: ActionDirect},
			{Enabled: true, MatchType: MatchDomainKeyword, Pattern: "proxy", Action: ActionProxy},
			{Enabled: true, MatchType: MatchIPCIDR, Pattern: "10.0.0.0/8", Action: ActionBlock},
		},
	}
	tests := []struct {
		host string
		want string
	}{
		{"exact.example", ActionBlock},
		{"example.com", ActionDirect},
		{"www.example.com", ActionDirect},
		{"a.b.example.com", ActionDirect},
		{"badexample.com", ActionProxy},
		{"use-proxy.test", ActionProxy},
		{"10.2.3.4", ActionBlock},
		{"unmatched.test", ActionProxy},
	}
	for _, test := range tests {
		if got := ResolveAction(config, test.host); got != test.want {
			t.Fatalf("ResolveAction(%q) = %q, want %q", test.host, got, test.want)
		}
	}
}

func TestDomainSuffixLeadingDotIsOptional(t *testing.T) {
	withoutDot := RoutingConfig{Mode: ModeRule, Rules: []Rule{{
		Enabled: true, MatchType: MatchDomainSuffix, Pattern: "cn", Action: ActionDirect,
	}}}
	withDot := RoutingConfig{Mode: ModeRule, Rules: []Rule{{
		Enabled: true, MatchType: MatchDomainSuffix, Pattern: ".cn", Action: ActionDirect,
	}}}
	for _, host := range []string{"cn", "example.cn", "a.b.example.cn"} {
		if got := ResolveAction(withoutDot, host); got != ActionDirect {
			t.Fatalf("suffix without dot did not match %q: %q", host, got)
		}
		if got := ResolveAction(withDot, host); got != ActionDirect {
			t.Fatalf("suffix with dot did not match %q: %q", host, got)
		}
	}
	if got := ResolveAction(withDot, "example.com"); got != ActionProxy {
		t.Fatalf("suffix .cn unexpectedly matched example.com: %q", got)
	}
}

func TestNormalizeRoutingConfigRejectsInvalidRules(t *testing.T) {
	got := NormalizeRoutingConfig(RoutingConfig{Mode: "bad", Rules: []Rule{
		{Enabled: true, MatchType: MatchDomain, Pattern: "", Action: ActionDirect},
		{Enabled: true, MatchType: "bad", Pattern: "example.com", Action: ActionDirect},
		{Enabled: true, MatchType: MatchDomain, Pattern: "example.com", Action: "bad"},
		{Enabled: true, MatchType: MatchDomain, Pattern: "example.com", Action: ActionDirect},
	}})
	if got.Mode != ModeProxy || len(got.Rules) != 1 {
		t.Fatalf("unexpected normalized config: %#v", got)
	}
}

func TestValidateRoutingConfig(t *testing.T) {
	valid := RoutingConfig{Mode: ModeRule, Rules: []Rule{{
		MatchType: MatchIPCIDR,
		Pattern:   "10.0.0.0/8",
		Action:    ActionDirect,
	}}}
	if err := ValidateRoutingConfig(valid); err != nil {
		t.Fatalf("valid routing rejected: %v", err)
	}

	invalid := []RoutingConfig{
		{Mode: "unknown"},
		{Mode: ModeRule, Rules: []Rule{{MatchType: MatchDomain, Pattern: "", Action: ActionProxy}}},
		{Mode: ModeRule, Rules: []Rule{{MatchType: "unknown", Pattern: "example.com", Action: ActionProxy}}},
		{Mode: ModeRule, Rules: []Rule{{MatchType: MatchDomain, Pattern: "example.com", Action: "unknown"}}},
		{Mode: ModeRule, Rules: []Rule{{MatchType: MatchIPCIDR, Pattern: "not-a-cidr", Action: ActionBlock}}},
	}
	for index, item := range invalid {
		if err := ValidateRoutingConfig(item); err == nil {
			t.Fatalf("invalid routing %d was accepted: %+v", index, item)
		}
	}
}
