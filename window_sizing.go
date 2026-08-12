package main

const (
	startupMinScaleNumerator   = 3
	startupMinScaleDenominator = 4
)

type desktopWorkArea struct {
	Width  int
	Height int
}

type startupWindowBounds struct {
	Width     int
	Height    int
	MinWidth  int
	MinHeight int
}

func resolveStartupWindowBounds(configBounds startupWindowBounds) startupWindowBounds {
	screenArea, hasScreenArea := getDesktopWorkArea()
	return fitStartupWindowBounds(configBounds, screenArea, hasScreenArea)
}

func fitStartupWindowBounds(configBounds startupWindowBounds, screenArea desktopWorkArea, hasScreenArea bool) startupWindowBounds {
	resolvedBounds := sanitizeStartupWindowBounds(configBounds)
	if hasScreenArea && screenArea.Width > 0 && screenArea.Height > 0 {
		resolvedBounds.Width = clampStartupDimension(resolvedBounds.Width, screenArea.Width)
		resolvedBounds.Height = clampStartupDimension(resolvedBounds.Height, screenArea.Height)
	}

	resolvedBounds.MinWidth = fitStartupMinimumDimension(resolvedBounds.MinWidth, resolvedBounds.Width)
	resolvedBounds.MinHeight = fitStartupMinimumDimension(resolvedBounds.MinHeight, resolvedBounds.Height)

	return resolvedBounds
}

func sanitizeStartupWindowBounds(bounds startupWindowBounds) startupWindowBounds {
	if bounds.Width <= 0 {
		bounds.Width = 1200
	}
	if bounds.Height <= 0 {
		bounds.Height = 800
	}
	if bounds.MinWidth < 0 {
		bounds.MinWidth = 0
	}
	if bounds.MinHeight < 0 {
		bounds.MinHeight = 0
	}
	return bounds
}

func clampStartupDimension(value int, maximum int) int {
	if maximum > 0 && value > maximum {
		return maximum
	}
	return value
}

func fitStartupMinimumDimension(configMinimum int, resolvedDimension int) int {
	if configMinimum <= 0 || resolvedDimension <= 0 {
		return 0
	}
	if configMinimum < resolvedDimension {
		return configMinimum
	}

	minimum := resolvedDimension * startupMinScaleNumerator / startupMinScaleDenominator
	if minimum <= 0 {
		return 1
	}
	return minimum
}
