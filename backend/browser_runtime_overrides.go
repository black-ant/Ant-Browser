package backend

import (
	"errors"
	"strings"
)

type browserRuntimeOverrides struct {
	Geolocation profileGeolocationOverride
	Timezone    profileTimezoneOverride
}

func (a *App) applyAndWatchBrowserRuntimeOverrides(profileId string, debugPort int, overrides browserRuntimeOverrides) error {
	var errs []error
	if err := a.applyAndWatchProfileGeolocation(profileId, debugPort, overrides.Geolocation); err != nil {
		errs = append(errs, err)
	}
	if err := a.applyAndWatchProfileTimezone(profileId, debugPort, overrides.Timezone); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (o browserRuntimeOverrides) geolocationSource() string {
	return strings.TrimSpace(o.Geolocation.Source)
}

func (o browserRuntimeOverrides) timezoneSource() string {
	return strings.TrimSpace(o.Timezone.Source)
}
