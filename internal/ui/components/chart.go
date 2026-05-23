package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ChartDatapoint is a single data point for a chart.
type ChartDatapoint struct {
	Value float64
	Label string // optional X-axis label (e.g. "14:00")
}

// Chart renders a sparkline/area chart using braille characters in a tview.TextView.
// Braille chars give 2x4 dot resolution per terminal cell (2 wide, 4 tall).
type Chart struct {
	*tview.TextView
	title  string
	unit   string
	data   []ChartDatapoint
	color  tcell.Color
	height int // chart area height in terminal rows
}

// NewChart creates a new chart widget.
func NewChart(title, unit string, color tcell.Color) *Chart {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetBorder(true)
	tv.SetBorderColor(tcell.ColorGray)
	tv.SetTitle(fmt.Sprintf(" %s ", title))
	tv.SetTitleColor(tcell.ColorWhite)

	return &Chart{
		TextView: tv,
		title:    title,
		unit:     unit,
		color:    color,
		height:   8, // default chart height
	}
}

// SetHeight sets the chart area height in terminal rows.
func (c *Chart) SetHeight(h int) {
	c.height = h
}

// SetData sets the data points and re-renders the chart.
func (c *Chart) SetData(data []ChartDatapoint) {
	c.data = data
	c.render()
}

// render draws the chart using braille characters.
func (c *Chart) render() {
	if len(c.data) == 0 {
		c.SetText("\n  [gray]No data available")
		return
	}

	// Find min/max for scaling
	minVal := math.MaxFloat64
	maxVal := -math.MaxFloat64
	for _, d := range c.data {
		if d.Value < minVal {
			minVal = d.Value
		}
		if d.Value > maxVal {
			maxVal = d.Value
		}
	}

	// Avoid division by zero
	valRange := maxVal - minVal
	if valRange == 0 {
		valRange = 1
		if maxVal > 0 {
			minVal = 0
		}
	}

	// Add some padding to the range
	padding := valRange * 0.05
	minVal -= padding
	if minVal < 0 && c.unit == "Percent" {
		minVal = 0
	}
	maxVal += padding
	if c.unit == "Percent" && maxVal > 100 {
		maxVal = 100
	}
	valRange = maxVal - minVal

	colorTag := colorTagFromTcell(c.color)

	// Braille-based rendering: each cell is 2 dots wide x 4 dots tall
	// Chart area height in dots = height * 4
	dotHeight := c.height * 4
	numPoints := len(c.data)

	// Build a grid: dotHeight rows x numPoints columns of booleans
	grid := make([][]bool, dotHeight)
	for y := range grid {
		grid[y] = make([]bool, numPoints)
	}

	// Fill the grid: for each data point, fill dots from bottom up to value
	for x, d := range c.data {
		normalised := (d.Value - minVal) / valRange
		dotY := int(normalised * float64(dotHeight-1))
		// Fill from bottom (dotHeight-1) up to the value level
		for y := dotHeight - 1; y >= dotHeight-1-dotY; y-- {
			if y >= 0 && y < dotHeight {
				grid[y][x] = true
			}
		}
	}

	// Convert grid to braille characters
	// Each braille cell covers 2 columns x 4 rows of dots
	brailleCols := (numPoints + 1) / 2
	brailleRows := (dotHeight + 3) / 4

	var b strings.Builder

	// Y-axis labels and chart
	for by := 0; by < brailleRows; by++ {
		// Y-axis label (max/mid/min at top/middle/bottom)
		if by == 0 {
			b.WriteString(fmt.Sprintf("[gray]%7s[%s] ", formatValue(maxVal, c.unit), colorTag))
		} else if by == brailleRows-1 {
			b.WriteString(fmt.Sprintf("[gray]%7s[%s] ", formatValue(minVal, c.unit), colorTag))
		} else if by == brailleRows/2 {
			mid := (maxVal + minVal) / 2
			b.WriteString(fmt.Sprintf("[gray]%7s[%s] ", formatValue(mid, c.unit), colorTag))
		} else {
			b.WriteString(fmt.Sprintf("       [%s] ", colorTag))
		}

		for bx := 0; bx < brailleCols; bx++ {
			b.WriteRune(brailleChar(grid, bx*2, by*4, dotHeight, numPoints))
		}
		b.WriteString("[-]\n")
	}

	// Current/latest value summary
	latest := c.data[len(c.data)-1]
	avg := 0.0
	for _, d := range c.data {
		avg += d.Value
	}
	avg /= float64(len(c.data))

	b.WriteString(fmt.Sprintf("[gray]        latest:[white] %s  [gray]avg:[white] %s  [gray]max:[white] %s  [gray]min:[white] %s",
		formatValue(latest.Value, c.unit),
		formatValue(avg, c.unit),
		formatValue(maxVal-padding, c.unit),
		formatValue(minVal+padding, c.unit)))

	c.SetText(b.String())
	c.SetTitle(fmt.Sprintf(" %s (%s) ", c.title, formatValue(latest.Value, c.unit)))
}

// brailleChar encodes a 2x4 dot region into a Unicode braille character.
// Braille encoding: dots are numbered:
//
//	0 3
//	1 4
//	2 5
//	6 7
//
// Unicode offset: 0x2800 + bit pattern
func brailleChar(grid [][]bool, startX, startY, maxY, maxX int) rune {
	// Dot positions within the braille cell (col, row) -> bit
	dotBits := [8][2]int{
		{0, 0}, // dot 0
		{0, 1}, // dot 1
		{0, 2}, // dot 2
		{1, 0}, // dot 3
		{1, 1}, // dot 4
		{1, 2}, // dot 5
		{0, 3}, // dot 6
		{1, 3}, // dot 7
	}

	var pattern int
	for bit, pos := range dotBits {
		x := startX + pos[0]
		y := startY + pos[1]
		if x < maxX && y < maxY && grid[y][x] {
			pattern |= 1 << bit
		}
	}

	if pattern == 0 {
		return ' '
	}
	return rune(0x2800 + pattern)
}

// formatValue formats a metric value with appropriate units.
func formatValue(v float64, unit string) string {
	switch unit {
	case "Percent":
		return fmt.Sprintf("%.1f%%", v)
	case "Bytes":
		return formatBytes(v)
	case "Count":
		if v >= 1_000_000 {
			return fmt.Sprintf("%.1fM", v/1_000_000)
		}
		if v >= 1_000 {
			return fmt.Sprintf("%.1fK", v/1_000)
		}
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

// formatBytes formats byte values into human-readable form.
func formatBytes(b float64) string {
	if b < 0 {
		b = 0
	}
	const unit = 1024.0
	if b < unit {
		return fmt.Sprintf("%.0f B", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KB", "MB", "GB", "TB", "PB"}
	if exp >= len(suffix) {
		exp = len(suffix) - 1
	}
	return fmt.Sprintf("%.1f %s", b/div, suffix[exp])
}

// colorTagFromTcell converts a tcell.Color to a tview dynamic color tag.
func colorTagFromTcell(c tcell.Color) string {
	switch c {
	case tcell.ColorGreen:
		return "green"
	case tcell.ColorRed:
		return "red"
	case tcell.ColorYellow:
		return "yellow"
	case tcell.ColorBlue:
		return "blue"
	case tcell.ColorDodgerBlue:
		return "dodgerblue"
	case tcell.ColorOrange:
		return "orange"
	case tcell.ColorPurple:
		return "purple"
	case tcell.ColorDarkCyan:
		return "cyan"
	case tcell.ColorDarkMagenta:
		return "magenta"
	default:
		return "white"
	}
}
