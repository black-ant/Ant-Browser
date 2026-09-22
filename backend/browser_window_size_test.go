package backend

import (
	"fmt"
	"reflect"
	"testing"
)

func TestConfiguredBrowserWindowSize(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantWidth  int
		wantHeight int
		wantOK     bool
	}{
		{name: "valid", args: []string{"--window-size=1440,1000"}, wantWidth: 1440, wantHeight: 1000, wantOK: true},
		{name: "last wins", args: []string{"--window-size=800,600", "--window-size= 1440,1000 "}, wantWidth: 1440, wantHeight: 1000, wantOK: true},
		{name: "missing", args: []string{"--lang=en-US"}},
		{name: "invalid", args: []string{"--window-size=wide,tall"}},
		{name: "non-positive", args: []string{"--window-size=1440,0"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			width, height, ok := configuredBrowserWindowSize(test.args)
			if width != test.wantWidth || height != test.wantHeight || ok != test.wantOK {
				t.Fatalf("configuredBrowserWindowSize(%v) = %d,%d,%v; want %d,%d,%v", test.args, width, height, ok, test.wantWidth, test.wantHeight, test.wantOK)
			}
		})
	}
}

func TestSyncConfiguredBrowserWindowSizeRestoresMinimizedStateAndDeduplicatesWindow(t *testing.T) {
	type callRecord struct {
		method string
		params map[string]any
	}
	var calls []callRecord
	call := func(_ int, method string, params map[string]any) (map[string]any, error) {
		calls = append(calls, callRecord{method: method, params: params})
		switch method {
		case "Target.getTargets":
			return map[string]any{"targetInfos": []any{
				map[string]any{"targetId": "page-1", "type": "page"},
				map[string]any{"targetId": "worker-1", "type": "service_worker"},
				map[string]any{"targetId": "page-2", "type": "page"},
			}}, nil
		case "Browser.getWindowForTarget":
			if len(calls) >= 5 {
				return map[string]any{
					"windowId": float64(42),
					"bounds": map[string]any{
						"width": float64(1440), "height": float64(1000), "windowState": "normal",
					},
				}, nil
			}
			return map[string]any{
				"windowId": float64(42),
				"bounds": map[string]any{
					"width": float64(2003), "height": float64(1152), "windowState": "minimized",
				},
			}, nil
		case "Browser.setWindowBounds":
			return map[string]any{}, nil
		default:
			return nil, fmt.Errorf("unexpected method %s", method)
		}
	}

	if err := syncConfiguredBrowserWindowSizeWithCall(44455, []string{"--window-size=1440,1000"}, call); err != nil {
		t.Fatal(err)
	}

	want := []callRecord{
		{method: "Target.getTargets", params: nil},
		{method: "Browser.getWindowForTarget", params: map[string]any{"targetId": "page-1"}},
		{method: "Browser.setWindowBounds", params: map[string]any{"windowId": 42, "bounds": map[string]any{"windowState": "normal"}}},
		{method: "Browser.setWindowBounds", params: map[string]any{"windowId": 42, "bounds": map[string]any{"width": 1440, "height": 1000}}},
		{method: "Browser.getWindowForTarget", params: map[string]any{"targetId": "page-1"}},
		{method: "Browser.setWindowBounds", params: map[string]any{"windowId": 42, "bounds": map[string]any{"windowState": "minimized"}}},
		{method: "Browser.getWindowForTarget", params: map[string]any{"targetId": "page-2"}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestSyncConfiguredBrowserWindowSizeLeavesMatchingNormalWindowUntouched(t *testing.T) {
	setCalls := 0
	call := func(_ int, method string, _ map[string]any) (map[string]any, error) {
		switch method {
		case "Target.getTargets":
			return map[string]any{"targetInfos": []any{map[string]any{"targetId": "page-1", "type": "page"}}}, nil
		case "Browser.getWindowForTarget":
			return map[string]any{"windowId": float64(7), "bounds": map[string]any{"width": float64(1440), "height": float64(1000), "windowState": "normal"}}, nil
		case "Browser.setWindowBounds":
			setCalls++
			return map[string]any{}, nil
		default:
			return nil, fmt.Errorf("unexpected method %s", method)
		}
	}

	if err := syncConfiguredBrowserWindowSizeWithCall(9222, []string{"--window-size=1440,1000"}, call); err != nil {
		t.Fatal(err)
	}
	if setCalls != 0 {
		t.Fatalf("Browser.setWindowBounds calls = %d, want 0", setCalls)
	}
}
