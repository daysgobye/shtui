package workbook

// Format identifies the file format used to load or save a workbook.
type Format string

const (
	CSV  Format = "csv"
	XLSX Format = "xlsx"
	XLSM Format = "xlsm"
)

// Coord is a zero-based row and column pair.
type Coord struct {
	Row int
	Col int
}

// CellRange is an inclusive rectangular range of cells.
type CellRange struct {
	Start Coord
	End   Coord
}

// BorderStyle describes one border edge.
type BorderStyle struct {
	Preset   string
	ColorRGB string
}

// BorderSet contains the four borders applied to a cell.
type BorderSet struct {
	Top    BorderStyle
	Right  BorderStyle
	Bottom BorderStyle
	Left   BorderStyle
}

// CellStyle stores direct formatting independent of the terminal theme.
type CellStyle struct {
	FillRGB      string
	FontRGB      string
	NumberFormat string
	Bold         bool
	Borders      BorderSet
}

// CalculationState distinguishes literals from formula results and their validity.
type CalculationState string

const (
	Literal     CalculationState = "literal"
	Current     CalculationState = "current"
	Stale       CalculationState = "stale"
	Unsupported CalculationState = "unsupported"
	Error       CalculationState = "error"
)

// Cell stores source input, the last calculated display result, and formatting.
type Cell struct {
	Input            string
	Result           string
	CalculationState CalculationState
	Style            CellStyle
}

// Sheet stores sparse cell content and sheet-level metadata.
type Sheet struct {
	Name         string
	Rows         int
	Cols         int
	Cells        map[Coord]Cell
	ColumnWidths map[int]float64
}

// Workbook is the editable in-memory representation of a spreadsheet.
type Workbook struct {
	Format      Format
	Path        string
	Sheets      []Sheet
	ActiveSheet int
	Dirty       bool
}
