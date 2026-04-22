package tui

const minWide = 90

// Layout describes the computed dimensions for a given terminal width.
type Layout struct {
	DashWidth int
	Wide      bool
	HalfWidth int
	BarWidth  int
}

// GetLayout computes the dashboard layout from terminal width.
// - 2-column at >= 90 cols, single-column below
// - Capped at 160 cols
// - barWidth = max(6, min(10, inner - 30)) where inner = halfWidth - 4
func GetLayout(termWidth int) Layout {
	dashWidth := termWidth
	if dashWidth > 160 {
		dashWidth = 160
	}
	if dashWidth < 40 {
		dashWidth = 40
	}
	wide := dashWidth >= minWide
	halfWidth := dashWidth
	if wide {
		halfWidth = dashWidth / 2
	}
	inner := halfWidth - 4
	barWidth := inner - 30
	if barWidth < 6 {
		barWidth = 6
	}
	if barWidth > 10 {
		barWidth = 10
	}
	return Layout{
		DashWidth: dashWidth,
		Wide:      wide,
		HalfWidth: halfWidth,
		BarWidth:  barWidth,
	}
}
