package tui

import "testing"

func TestGetLayout_WideBreakpoint(t *testing.T) {
	narrow := GetLayout(80)
	if narrow.Wide {
		t.Error("width=80 should be single-column (not wide)")
	}

	wide := GetLayout(90)
	if !wide.Wide {
		t.Error("width=90 should be two-column (wide)")
	}

	wide2 := GetLayout(100)
	if !wide2.Wide {
		t.Error("width=100 should be wide")
	}
}

func TestGetLayout_Cap(t *testing.T) {
	l := GetLayout(200)
	if l.DashWidth != 160 {
		t.Errorf("expected DashWidth=160 (capped), got %d", l.DashWidth)
	}
}

func TestGetLayout_HalfWidth(t *testing.T) {
	l := GetLayout(120)
	if !l.Wide {
		t.Error("width=120 should be wide")
	}
	if l.HalfWidth != 60 {
		t.Errorf("HalfWidth = %d, want 60", l.HalfWidth)
	}
}

func TestGetLayout_BarWidth_Clamp(t *testing.T) {
	// Very narrow: barWidth should be clamped at 6
	narrow := GetLayout(40)
	if narrow.BarWidth < 6 {
		t.Errorf("BarWidth below minimum: got %d", narrow.BarWidth)
	}

	// Very wide: barWidth should be clamped at 10
	wide := GetLayout(160)
	if wide.BarWidth > 10 {
		t.Errorf("BarWidth above maximum: got %d", wide.BarWidth)
	}
}

func TestGetLayout_SingleColumn(t *testing.T) {
	l := GetLayout(80)
	if l.Wide {
		t.Error("expected single-column for 80 width")
	}
	if l.HalfWidth != l.DashWidth {
		t.Errorf("single-column: HalfWidth should equal DashWidth, got %d vs %d", l.HalfWidth, l.DashWidth)
	}
}
