//go:build !windows
// +build !windows

package backend

func profileWindowMarkerSupported() bool {
	return false
}

func runProfileWindowMarker(_ *App, marker *profileWindowMarker) {
	<-marker.stop
}
