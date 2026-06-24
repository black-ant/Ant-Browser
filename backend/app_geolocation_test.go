package backend

import (
	"reflect"
	"testing"
)

func TestExtractProfileGeolocationArgs(t *testing.T) {
	t.Parallel()

	filtered, geo, warnings := extractProfileGeolocationArgs([]string{
		"--fingerprint=123",
		"--ant-geolocation=0,-74.006,50",
		"--ant-geolocation-permission=allow",
		"--lang=en-US",
	})

	wantFiltered := []string{"--fingerprint=123", "--lang=en-US"}
	if !reflect.DeepEqual(filtered, wantFiltered) {
		t.Fatalf("filtered args mismatch: got=%v want=%v", filtered, wantFiltered)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got=%v", warnings)
	}
	if !geo.Explicit || !geo.HasPosition || geo.Permission != "allow" || geo.Source != "profile" {
		t.Fatalf("unexpected geolocation metadata: %+v", geo)
	}
	if geo.Latitude != 0 || geo.Longitude != -74.006 || geo.Accuracy != 50 {
		t.Fatalf("unexpected geolocation coordinates: %+v", geo)
	}
}

func TestExtractProfileGeolocationArgsStripsInvalidInternalArgs(t *testing.T) {
	t.Parallel()

	filtered, geo, warnings := extractProfileGeolocationArgs([]string{
		"--ant-geolocation=200,10",
		"--disable-sync",
	})

	if !reflect.DeepEqual(filtered, []string{"--disable-sync"}) {
		t.Fatalf("filtered args mismatch: got=%v", filtered)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got=%v", warnings)
	}
	if !geo.Explicit || geo.HasPosition {
		t.Fatalf("invalid internal arg should be explicit but not applicable: %+v", geo)
	}
}

func TestExtractProfileGeolocationArgsTreatsModeAsExplicit(t *testing.T) {
	t.Parallel()

	filtered, geo, warnings := extractProfileGeolocationArgs([]string{
		"--ant-geolocation-mode=real",
		"--disable-sync",
	})

	if !reflect.DeepEqual(filtered, []string{"--disable-sync"}) {
		t.Fatalf("filtered args mismatch: got=%v", filtered)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got=%v", warnings)
	}
	if !geo.Explicit || geo.HasPosition || geo.shouldApply() {
		t.Fatalf("real mode should suppress proxy geolocation without applying override: %+v", geo)
	}
}

func TestProxyConsistencyControlsSkipTimezone(t *testing.T) {
	t.Parallel()

	filtered, controls := extractProxyConsistencyControlArgs([]string{
		"--ant-timezone-mode=real",
		"--disable-sync",
	})
	if !reflect.DeepEqual(filtered, []string{"--disable-sync"}) {
		t.Fatalf("filtered args mismatch: got=%v", filtered)
	}
	if !controls.SkipTimezone {
		t.Fatal("expected real timezone mode to skip proxy timezone injection")
	}

	args := buildProxyConsistencyArgs(nil, `{"ok":true,"country":"US","region":"New York","rawData":{"timezone":"America/New_York"}}`, controls)
	for _, arg := range args {
		if arg == "--timezone=America/New_York" {
			t.Fatalf("timezone should not be injected when skipped: %v", args)
		}
	}
}

func TestProxyConsistencyArgsInjectAcceptLanguage(t *testing.T) {
	t.Parallel()

	args := buildProxyConsistencyArgs(nil, `{"ok":true,"country":"US","region":"New York","rawData":{"timezone":"America/New_York"}}`)
	if !reflect.DeepEqual(args, []string{
		"--timezone=America/New_York",
		"--lang=en-US",
		"--accept-language=en-US,en",
		"--webrtc-ip-handling-policy=disable_non_proxied_udp",
	}) {
		t.Fatalf("proxy consistency args mismatch: got=%v", args)
	}
}

func TestProxyConsistencyArgsPreservesExplicitAcceptLanguage(t *testing.T) {
	t.Parallel()

	args := buildProxyConsistencyArgs([]string{"--accept-language=ja-JP,ja"}, `{"ok":true,"country":"US","region":"New York"}`)
	for _, arg := range args {
		if arg == "--accept-language=en-US,en" {
			t.Fatalf("explicit accept-language should not be overwritten: %v", args)
		}
	}
}

func TestProxyGeolocationOverride(t *testing.T) {
	t.Parallel()

	geo, ok := proxyGeolocationOverride(`{"ok":true,"country":"US","region":"NY","rawData":{"latitude":40.71,"longitude":-74.0}}`)
	if !ok {
		t.Fatal("expected proxy geolocation override")
	}
	if !geo.HasPosition || geo.Latitude != 40.71 || geo.Longitude != -74.0 || geo.Accuracy != 100 {
		t.Fatalf("unexpected proxy geolocation override: %+v", geo)
	}
	if geo.Permission != "prompt" || geo.Source != "proxy" || geo.Explicit {
		t.Fatalf("unexpected proxy geolocation metadata: %+v", geo)
	}
}

func TestGeolocationCDPParams(t *testing.T) {
	t.Parallel()

	params := profileGeolocationOverride{
		Latitude:  35.6895,
		Longitude: 139.6917,
		Accuracy:  25,
	}.cdpParams()

	if params["latitude"] != 35.6895 || params["longitude"] != 139.6917 || params["accuracy"] != float64(25) {
		t.Fatalf("unexpected geolocation params: %+v", params)
	}
}

func TestAttachedPageSessionID(t *testing.T) {
	t.Parallel()

	sessionID, ok := attachedPageSessionID(targetAttachCDPMessage{
		Method: "Target.attachedToTarget",
		Params: map[string]any{
			"sessionId": "session-1",
			"targetInfo": map[string]any{
				"type": "page",
			},
		},
	})
	if !ok || sessionID != "session-1" {
		t.Fatalf("expected page session, got session=%q ok=%v", sessionID, ok)
	}
}

func TestAttachedPageSessionIDIgnoresNonPageTargets(t *testing.T) {
	t.Parallel()

	sessionID, ok := attachedPageSessionID(targetAttachCDPMessage{
		Method: "Target.attachedToTarget",
		Params: map[string]any{
			"sessionId": "session-1",
			"targetInfo": map[string]any{
				"type": "service_worker",
			},
		},
	})
	if ok || sessionID != "" {
		t.Fatalf("expected non-page target to be ignored, got session=%q ok=%v", sessionID, ok)
	}
}
