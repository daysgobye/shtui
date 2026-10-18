package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type RGB struct {
	R uint8
	G uint8
	B uint8
}

type ColorOption struct {
	Name    string
	ANSI    uint8
	Color   RGB
	Custom  bool
	Default bool
}

type ThemePalette struct {
	custom []RGB
}

var standardANSIColors = [16]RGB{
	{R: 0, G: 0, B: 0},
	{R: 205, G: 0, B: 0},
	{R: 0, G: 205, B: 0},
	{R: 205, G: 205, B: 0},
	{R: 0, G: 0, B: 238},
	{R: 205, G: 0, B: 205},
	{R: 0, G: 205, B: 205},
	{R: 229, G: 229, B: 229},
	{R: 127, G: 127, B: 127},
	{R: 255, G: 0, B: 0},
	{R: 0, G: 255, B: 0},
	{R: 255, G: 255, B: 0},
	{R: 92, G: 92, B: 255},
	{R: 255, G: 0, B: 255},
	{R: 0, G: 255, B: 255},
	{R: 255, G: 255, B: 255},
}

var ansiNames = [16]string{
	"Black", "Red", "Green", "Yellow", "Blue", "Magenta", "Cyan", "White",
	"Bright black", "Bright red", "Bright green", "Bright yellow", "Bright blue", "Bright magenta", "Bright cyan", "Bright white",
}

func NewThemePalette() ThemePalette {
	return ThemePalette{}
}

// Options always lists terminal theme slots before user-provided RGB colors.
func (palette ThemePalette) Options() []ColorOption {
	options := make([]ColorOption, 0, len(standardANSIColors)+1+len(palette.custom))
	options = append(options, ColorOption{Name: "Terminal default", Default: true})
	for index, color := range standardANSIColors {
		options = append(options, ColorOption{Name: ansiNames[index], ANSI: uint8(index), Color: color})
	}
	for index, color := range palette.custom {
		options = append(options, ColorOption{Name: FormatRGBHex(color), Color: color, Custom: true, ANSI: uint8(index)})
	}
	return options
}

func (palette *ThemePalette) AddCustom(value string) error {
	if palette == nil {
		return fmt.Errorf("add custom color: palette is nil")
	}
	color, err := ParseRGBHex(value)
	if err != nil {
		return err
	}
	for _, existing := range palette.custom {
		if existing == color {
			return nil
		}
	}
	palette.custom = append(palette.custom, color)
	return nil
}

func ParseRGBHex(value string) (RGB, error) {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return RGB{}, fmt.Errorf("color %q must use #RRGGBB format", value)
	}
	red, err := strconv.ParseUint(value[1:3], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid red channel in color %q", value)
	}
	green, err := strconv.ParseUint(value[3:5], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid green channel in color %q", value)
	}
	blue, err := strconv.ParseUint(value[5:7], 16, 8)
	if err != nil {
		return RGB{}, fmt.Errorf("invalid blue channel in color %q", value)
	}
	return RGB{R: uint8(red), G: uint8(green), B: uint8(blue)}, nil
}

func FormatRGBHex(color RGB) string {
	return fmt.Sprintf("#%02X%02X%02X", color.R, color.G, color.B)
}

func NearestANSI(color RGB) uint8 {
	bestIndex := uint8(0)
	bestDistance := int(^uint(0) >> 1)
	for index, candidate := range standardANSIColors {
		red := int(color.R) - int(candidate.R)
		green := int(color.G) - int(candidate.G)
		blue := int(color.B) - int(candidate.B)
		distance := red*red + green*green + blue*blue
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = uint8(index)
		}
	}
	return bestIndex
}

// ResolveThemeColor queries a terminal's ANSI palette only for an assigned cell.
func ResolveThemeColor(index uint8, query func(uint8, time.Duration) (RGB, bool)) RGB {
	if query == nil {
		return standardANSIColors[index%uint8(len(standardANSIColors))]
	}
	type result struct {
		color RGB
		ok    bool
	}
	completed := make(chan result, 1)
	go func() {
		color, ok := query(index, 100*time.Millisecond)
		completed <- result{color: color, ok: ok}
	}()
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case value := <-completed:
		if value.ok {
			return value.color
		}
	case <-timer.C:
	}
	return standardANSIColors[index%uint8(len(standardANSIColors))]
}

func ANSIColor(index uint8) lipgloss.Color {
	return lipgloss.Color(strconv.Itoa(int(index)))
}

func TerminalColor(color RGB, trueColor bool) lipgloss.TerminalColor {
	if !trueColor {
		return ANSIColor(NearestANSI(color))
	}
	return lipgloss.Color(FormatRGBHex(color))
}
