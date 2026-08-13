package raptor

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ANSI color and format codes
const (
	ansiReset         = "\033[0m"
	ansiBold          = "\033[1m"
	ansiDim           = "\033[2m"
	ansiItalic        = "\033[3m"
	ansiUnderline     = "\033[4m"
	ansiStrikethrough = "\033[9m"
)

// Predefined 16-color ANSI map
var namedColors = map[string]int{
	"black":   0,
	"red":     1,
	"green":   2,
	"yellow":  3,
	"blue":    4,
	"magenta": 5,
	"purple":  5,
	"cyan":    6,
	"white":   7,
	"gray":    8,
	"grey":    8,
	"orange":  208,
	"pink":    205,
	"slate":   244,
	"sky":     117,
	"emerald": 48,
	"rose":    204,
	"amber":   214,
	"violet":  135,
}

// parseHexColor converts "#RRGGBB" or "#RGB" into 24-bit ANSI RGB string
func parseHexColor(hex string, isBg bool) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = fmt.Sprintf("%c%c%c%c%c%c", hex[0], hex[0], hex[1], hex[1], hex[2], hex[2])
	}
	if len(hex) != 6 {
		return ""
	}
	r, err1 := strconv.ParseInt(hex[0:2], 16, 64)
	g, err2 := strconv.ParseInt(hex[2:4], 16, 64)
	b, err3 := strconv.ParseInt(hex[4:6], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	if isBg {
		return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
	}
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

// colorToANSI converts color name, hex code, or 256-color number to ANSI escape
func colorToANSI(cVal *Value, isBg bool) string {
	if cVal == nil || cVal.Type == ValNil {
		return ""
	}
	if cVal.Type == ValInt {
		code := cVal.IntVal
		if isBg {
			return fmt.Sprintf("\033[48;5;%dm", code)
		}
		return fmt.Sprintf("\033[38;5;%dm", code)
	}
	str := strings.TrimSpace(strings.ToLower(cVal.String()))
	if strings.HasPrefix(str, "#") {
		return parseHexColor(str, isBg)
	}
	if idx, ok := namedColors[str]; ok {
		if idx < 8 {
			base := 30
			if isBg {
				base = 40
			}
			return fmt.Sprintf("\033[%dm", base+idx)
		}
		if isBg {
			return fmt.Sprintf("\033[48;5;%dm", idx)
		}
		return fmt.Sprintf("\033[38;5;%dm", idx)
	}
	return ""
}

// Border characters
type BorderStyle struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
}

var (
	BorderRounded = BorderStyle{"╭", "╮", "╰", "╯", "─", "│"}
	BorderNormal  = BorderStyle{"┌", "┐", "└", "┘", "─", "│"}
	BorderDouble  = BorderStyle{"╔", "╗", "╚", "╝", "═", "║"}
	BorderThick   = BorderStyle{"┏", "┓", "┗", "┛", "━", "┃"}
	BorderHidden  = BorderStyle{" ", " ", " ", " ", " ", " "}
)

func getBorderStyle(name string) BorderStyle {
	switch strings.ToLower(name) {
	case "rounded":
		return BorderRounded
	case "double":
		return BorderDouble
	case "thick":
		return BorderThick
	case "hidden":
		return BorderHidden
	default:
		return BorderNormal
	}
}

// registerTUIBuiltins registers all Charmbracelet / Lip Gloss styling and TUI functions
func registerTUIBuiltins(in *Interp) {
	// tui_style($text, %opts) -> styled ANSI string
	in.Builtins["tui_style"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return StringValue(""), nil
		}
		text := args[0].String()
		opts := make(map[string]*Value)
		if len(args) > 1 && args[1].Type == ValHash {
			opts = args[1].HashVal
		}

		var prefix strings.Builder
		var suffix strings.Builder

		if opt, ok := opts["bold"]; ok && opt.IsTrue() {
			prefix.WriteString(ansiBold)
		}
		if opt, ok := opts["dim"]; ok && opt.IsTrue() {
			prefix.WriteString(ansiDim)
		}
		if opt, ok := opts["italic"]; ok && opt.IsTrue() {
			prefix.WriteString(ansiItalic)
		}
		if opt, ok := opts["underline"]; ok && opt.IsTrue() {
			prefix.WriteString(ansiUnderline)
		}
		if opt, ok := opts["strikethrough"]; ok && opt.IsTrue() {
			prefix.WriteString(ansiStrikethrough)
		}

		if opt, ok := opts["fg"]; ok {
			prefix.WriteString(colorToANSI(opt, false))
		}
		if opt, ok := opts["bg"]; ok {
			prefix.WriteString(colorToANSI(opt, true))
		}

		if prefix.Len() > 0 {
			suffix.WriteString(ansiReset)
		}

		// Width and alignment
		width := 0
		if opt, ok := opts["width"]; ok {
			width = int(in.toInt(opt))
		}
		align := "left"
		if opt, ok := opts["align"]; ok {
			align = strings.ToLower(opt.String())
		}

		lines := strings.Split(text, "\n")
		var styledLines []string
		for _, line := range lines {
			lineLen := utf8.RuneCountInString(stripANSI(line))
			if width > lineLen {
				pad := width - lineLen
				switch align {
				case "right":
					line = strings.Repeat(" ", pad) + line
				case "center":
					leftPad := pad / 2
					rightPad := pad - leftPad
					line = strings.Repeat(" ", leftPad) + line + strings.Repeat(" ", rightPad)
				default:
					line = line + strings.Repeat(" ", pad)
				}
			}
			styledLines = append(styledLines, prefix.String()+line+suffix.String())
		}

		res := strings.Join(styledLines, "\n")

		// Framing border if requested
		if opt, ok := opts["border"]; ok && opt.IsTrue() {
			borderName := "rounded"
			if opt.Type == ValString {
				borderName = opt.StrVal
			}
			borderFg := ""
			if bOpt, ok := opts["border_fg"]; ok {
				borderFg = colorToANSI(bOpt, false)
			}
			title := ""
			if tOpt, ok := opts["title"]; ok {
				title = tOpt.String()
			}
			res = renderBox(res, borderName, borderFg, title)
		}

		return StringValue(res), nil
	}

	// tui_box($text, %opts) -> framed box string
	in.Builtins["tui_box"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return StringValue(""), nil
		}
		text := args[0].String()
		borderStyle := "rounded"
		borderFg := ""
		title := ""

		if len(args) > 1 && args[1].Type == ValHash {
			opts := args[1].HashVal
			if opt, ok := opts["border"]; ok {
				borderStyle = opt.String()
			}
			if opt, ok := opts["border_fg"]; ok {
				borderFg = colorToANSI(opt, false)
			}
			if opt, ok := opts["title"]; ok {
				title = opt.String()
			}
		}

		return StringValue(renderBox(text, borderStyle, borderFg, title)), nil
	}

	// tui_table(@headers, @rows, %opts) -> styled table string
	in.Builtins["tui_table"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return StringValue(""), nil
		}
		var headers []string
		if args[0].Type == ValArray {
			for _, h := range args[0].ArrayVal {
				headers = append(headers, h.String())
			}
		}

		var rows [][]string
		if args[1].Type == ValArray {
			for _, r := range args[1].ArrayVal {
				var row []string
				if r.Type == ValArray {
					for _, cell := range r.ArrayVal {
						row = append(row, cell.String())
					}
				}
				rows = append(rows, row)
			}
		}

		return StringValue(renderTable(headers, rows)), nil
	}

	// tui_progress($percent, %opts) -> progress bar string
	in.Builtins["tui_progress"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return StringValue(""), nil
		}
		pct := in.toFloat(args[0])
		if pct < 0 {
			pct = 0
		}
		if pct > 1.0 && pct <= 100.0 {
			pct = pct / 100.0
		}
		if pct > 1.0 {
			pct = 1.0
		}

		width := 30
		fg := "\033[38;2;56;189;248m" // Sky blue
		if len(args) > 1 && args[1].Type == ValHash {
			opts := args[1].HashVal
			if opt, ok := opts["width"]; ok {
				width = int(in.toInt(opt))
			}
			if opt, ok := opts["fg"]; ok {
				fg = colorToANSI(opt, false)
			}
		}

		filledCount := int(math.Round(float64(width) * pct))
		if filledCount > width {
			filledCount = width
		}
		emptyCount := width - filledCount

		filled := strings.Repeat("█", filledCount)
		empty := strings.Repeat("░", emptyCount)

		pctStr := fmt.Sprintf(" %3.0f%%", pct*100)
		return StringValue(fg + filled + "\033[0m\033[2m" + empty + ansiReset + pctStr), nil
	}

	// tui_markdown($md) -> ANSI terminal styled markdown
	in.Builtins["tui_markdown"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 {
			return StringValue(""), nil
		}
		md := args[0].String()
		return StringValue(renderMarkdown(md)), nil
	}

	// tui_clear() -> clears terminal screen
	in.Builtins["tui_clear"] = func(in *Interp, args []*Value) (*Value, error) {
		fmt.Print("\033[2J\033[H")
		return BoolValue(true), nil
	}

	// tui_app_run(%spec) -> Elm architecture event loop
	in.Builtins["tui_app_run"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) == 0 || args[0].Type != ValHash {
			return nil, fmt.Errorf("tui_app_run requires a configuration hash with :init, :update, :view")
		}
		spec := args[0].HashVal

		// Initialize model
		var model *Value = NilValue()
		if initFn, ok := spec["init"]; ok && initFn.Type == ValClosure {
			res, err := in.InvokeCallable(initFn, nil)
			if err != nil {
				return nil, err
			}
			model = res
		}

		// Initial view
		if viewFn, ok := spec["view"]; ok && viewFn.Type == ValClosure {
			out, err := in.InvokeCallable(viewFn, []*Value{model})
			if err != nil {
				return nil, err
			}
			fmt.Println(out.String())
		}

		return model, nil
	}
}

// renderBox formats text inside a border box
func renderBox(text string, styleName, borderFg, title string) string {
	b := getBorderStyle(styleName)
	lines := strings.Split(text, "\n")

	maxW := 0
	for _, l := range lines {
		w := utf8.RuneCountInString(stripANSI(l))
		if w > maxW {
			maxW = w
		}
	}
	if maxW < 10 {
		maxW = 10
	}

	borderReset := ""
	if borderFg != "" {
		borderReset = ansiReset
	}

	var sb strings.Builder

	// Top border
	sb.WriteString(borderFg + b.TopLeft)
	if title != "" {
		tLen := utf8.RuneCountInString(title)
		if tLen+4 <= maxW+2 {
			sb.WriteString(" " + ansiBold + title + ansiReset + borderFg + " ")
			rem := (maxW + 2) - (tLen + 4)
			sb.WriteString(strings.Repeat(b.Horizontal, rem))
		} else {
			sb.WriteString(strings.Repeat(b.Horizontal, maxW+2))
		}
	} else {
		sb.WriteString(strings.Repeat(b.Horizontal, maxW+2))
	}
	sb.WriteString(b.TopRight + borderReset + "\n")

	// Content lines
	for _, l := range lines {
		w := utf8.RuneCountInString(stripANSI(l))
		pad := maxW - w
		sb.WriteString(borderFg + b.Vertical + borderReset + " " + l + strings.Repeat(" ", pad) + " " + borderFg + b.Vertical + borderReset + "\n")
	}

	// Bottom border
	sb.WriteString(borderFg + b.BottomLeft + strings.Repeat(b.Horizontal, maxW+2) + b.BottomRight + borderReset)
	return sb.String()
}

// renderTable formats headers and rows into ASCII table
func renderTable(headers []string, rows [][]string) string {
	cols := len(headers)
	if cols == 0 && len(rows) > 0 {
		cols = len(rows[0])
	}
	if cols == 0 {
		return ""
	}

	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(stripANSI(h))
	}
	for _, r := range rows {
		for i, cell := range r {
			if i < cols {
				w := utf8.RuneCountInString(stripANSI(cell))
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	var sb strings.Builder

	// Header row
	if len(headers) > 0 {
		sb.WriteString("\033[1;38;2;56;189;248m") // Bold Sky blue
		for i, h := range headers {
			pad := widths[i] - utf8.RuneCountInString(stripANSI(h))
			sb.WriteString("  " + h + strings.Repeat(" ", pad) + "  ")
			if i < cols-1 {
				sb.WriteString("│")
			}
		}
		sb.WriteString(ansiReset + "\n")

		// Separator
		for i := range headers {
			sb.WriteString(strings.Repeat("─", widths[i]+4))
			if i < cols-1 {
				sb.WriteString("┼")
			}
		}
		sb.WriteString("\n")
	}

	// Data rows
	for rIdx, r := range rows {
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(r) {
				cell = r[i]
			}
			pad := widths[i] - utf8.RuneCountInString(stripANSI(cell))
			if rIdx%2 == 1 {
				sb.WriteString("\033[2m") // Dim odd rows
			}
			sb.WriteString("  " + cell + strings.Repeat(" ", pad) + "  " + ansiReset)
			if i < cols-1 {
				sb.WriteString("│")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderMarkdown converts markdown to ANSI formatted terminal text
func renderMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	var sb strings.Builder
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code blocks
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				sb.WriteString("\033[38;5;244m─── Code ───\033[0m\n")
			} else {
				sb.WriteString("\033[38;5;244m────────────\033[0m\n")
			}
			continue
		}
		if inCodeBlock {
			sb.WriteString("  \033[38;2;250;204;21m" + line + ansiReset + "\n") // Yellow code
			continue
		}

		// Headers
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimPrefix(line, "# ")
			sb.WriteString("\n\033[1;38;2;56;189;248m" + strings.ToUpper(title) + ansiReset + "\n")
			sb.WriteString("\033[38;2;56;189;248m" + strings.Repeat("━", utf8.RuneCountInString(title)) + ansiReset + "\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			title := strings.TrimPrefix(line, "## ")
			sb.WriteString("\n\033[1;38;2;244;63;94m## " + title + ansiReset + "\n")
			continue
		}
		if strings.HasPrefix(line, "### ") {
			title := strings.TrimPrefix(line, "### ")
			sb.WriteString("\n\033[1;38;2;74;222;128m### " + title + ansiReset + "\n")
			continue
		}

		// Blockquotes
		if strings.HasPrefix(trimmed, "> ") {
			quote := strings.TrimPrefix(trimmed, "> ")
			sb.WriteString("  \033[38;5;244m│ \033[3m" + quote + ansiReset + "\n")
			continue
		}

		// Bullet lists
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			item := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			sb.WriteString("  \033[38;2;56;189;248m•\033[0m " + item + "\n")
			continue
		}

		// Regular line
		sb.WriteString(line + "\n")
	}

	return sb.String()
}

// stripANSI removes ANSI escape codes to calculate visual character length
func stripANSI(s string) string {
	var sb strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
