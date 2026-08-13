package raptor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// PodNodeType identifies kind of POD element.
type PodNodeType int

const (
	PodNodeProse PodNodeType = iota
	PodNodeHeading
	PodNodeItem
	PodNodeChunk
	PodNodeVerbatim
)

// CodeChunk represents a named code snippet in a literate document.
type CodeChunk struct {
	Name       string
	FileTarget string
	Mangles    []string
	Lines      []string
	LineNumber int
}

// PodSection represents a prose section or code chunk in POD.
type PodSection struct {
	Type       PodNodeType
	Level      int      // for headings: 1..6
	Heading    string   // heading title
	Content    []string // prose text lines
	Chunk      *CodeChunk
	LineNumber int
}

// PodDoc represents a parsed Literate POD document.
type PodDoc struct {
	Sections []*PodSection
	Chunks   map[string]*CodeChunk
}

var (
	chunkMacroRe  = regexp.MustCompile(`(<<\s*([a-zA-Z0-9_\-\.\/]+)\s*>>|«\s*([a-zA-Z0-9_\-\.\/]+)\s*»)`)
	podFormatBRe  = regexp.MustCompile(`B<([^>]+)>`)
	podFormatIRe  = regexp.MustCompile(`I<([^>]+)>`)
	podFormatCRe  = regexp.MustCompile(`C<([^>]+)>`)
	podFormatLRe  = regexp.MustCompile(`L<([^>]+)>`)
)

// ParsePodDoc parses raw POD / Literate POD source into a PodDoc AST.
func ParsePodDoc(source string) (*PodDoc, error) {
	lines := strings.Split(source, "\n")
	doc := &PodDoc{
		Sections: make([]*PodSection, 0),
		Chunks:   make(map[string]*CodeChunk),
	}

	inPod := false
	var currentSection *PodSection
	var currentChunk *CodeChunk

	for i, rawLine := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(rawLine)

		// Check POD command
		if strings.HasPrefix(trimmed, "=") {
			parts := strings.Fields(trimmed)
			cmd := parts[0]

			switch {
			case cmd == "=pod":
				inPod = true
				continue

			case cmd == "=cut":
				inPod = false
				if currentChunk != nil {
					doc.Chunks[currentChunk.Name] = currentChunk
					currentChunk = nil
				}
				currentSection = nil
				continue

			case strings.HasPrefix(cmd, "=head"):
				inPod = true
				if currentChunk != nil {
					doc.Chunks[currentChunk.Name] = currentChunk
					currentChunk = nil
				}
				level := 1
				if len(cmd) > 5 {
					if l, err := strconv.Atoi(cmd[5:]); err == nil {
						level = l
					}
				}
				headingText := strings.TrimSpace(strings.TrimPrefix(trimmed, cmd))
				currentSection = &PodSection{
					Type:       PodNodeHeading,
					Level:      level,
					Heading:    headingText,
					LineNumber: lineNum,
				}
				doc.Sections = append(doc.Sections, currentSection)
				continue

			case cmd == "=item":
				inPod = true
				itemText := strings.TrimSpace(strings.TrimPrefix(trimmed, cmd))
				currentSection = &PodSection{
					Type:       PodNodeItem,
					Content:    []string{itemText},
					LineNumber: lineNum,
				}
				doc.Sections = append(doc.Sections, currentSection)
				continue

			case cmd == "=over" || cmd == "=back":
				continue

			case cmd == "=chunk" || cmd == "=code" || (cmd == "=begin" && len(parts) > 1 && (parts[1] == "chunk" || parts[1] == "code")):
				inPod = true
				chunkName, fileTarget, mangles := parseChunkHeader(trimmed)
				currentChunk = &CodeChunk{
					Name:       chunkName,
					FileTarget: fileTarget,
					Mangles:    mangles,
					Lines:      make([]string, 0),
					LineNumber: lineNum,
				}
				currentSection = &PodSection{
					Type:       PodNodeChunk,
					Chunk:      currentChunk,
					LineNumber: lineNum,
				}
				doc.Sections = append(doc.Sections, currentSection)
				continue

			case cmd == "=end":
				if currentChunk != nil {
					doc.Chunks[currentChunk.Name] = currentChunk
					currentChunk = nil
				}
				currentSection = nil
				continue
			}
		}

		// Body content
		if currentChunk != nil {
			if strings.HasPrefix(trimmed, "=end") || trimmed == "=cut" {
				doc.Chunks[currentChunk.Name] = currentChunk
				currentChunk = nil
				currentSection = nil
			} else {
				currentChunk.Lines = append(currentChunk.Lines, rawLine)
			}
		} else if inPod {
			if currentSection == nil || currentSection.Type != PodNodeProse {
				currentSection = &PodSection{
					Type:       PodNodeProse,
					Content:    make([]string, 0),
					LineNumber: lineNum,
				}
				doc.Sections = append(doc.Sections, currentSection)
			}
			currentSection.Content = append(currentSection.Content, rawLine)
		} else {
			// Outside POD block: check if code is verbatim or standalone chunk
			if trimmed != "" {
				if currentSection == nil || currentSection.Type != PodNodeVerbatim {
					currentSection = &PodSection{
						Type:       PodNodeVerbatim,
						Content:    make([]string, 0),
						LineNumber: lineNum,
					}
					doc.Sections = append(doc.Sections, currentSection)
				}
				currentSection.Content = append(currentSection.Content, rawLine)
			}
		}
	}

	if currentChunk != nil {
		doc.Chunks[currentChunk.Name] = currentChunk
	}

	return doc, nil
}

func parseChunkHeader(headerLine string) (name string, fileTarget string, mangles []string) {
	headerLine = strings.TrimPrefix(headerLine, "=begin ")
	headerLine = strings.TrimPrefix(headerLine, "=chunk")
	headerLine = strings.TrimPrefix(headerLine, "=code")
	headerLine = strings.TrimSpace(headerLine)

	// Extract name: e.g. <chunk-name> or "chunk-name" or chunk-name
	if strings.HasPrefix(headerLine, "<") {
		idx := strings.Index(headerLine, ">")
		if idx != -1 {
			name = headerLine[1:idx]
			headerLine = strings.TrimSpace(headerLine[idx+1:])
		}
	} else if strings.HasPrefix(headerLine, "\"") {
		idx := strings.Index(headerLine[1:], "\"")
		if idx != -1 {
			name = headerLine[1 : idx+1]
			headerLine = strings.TrimSpace(headerLine[idx+2:])
		}
	} else {
		parts := strings.Fields(headerLine)
		if len(parts) > 0 && !strings.HasPrefix(parts[0], ":") {
			name = parts[0]
			headerLine = strings.TrimSpace(strings.TrimPrefix(headerLine, name))
		}
	}

	if name == "" {
		name = "main"
	}

	// Parse options: :file "path", :mangle(a, b, c)
	if strings.Contains(headerLine, ":file") {
		idx := strings.Index(headerLine, ":file")
		rest := strings.TrimSpace(headerLine[idx+5:])
		rest = strings.TrimPrefix(rest, "=")
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, "\"") {
			endQuote := strings.Index(rest[1:], "\"")
			if endQuote != -1 {
				fileTarget = rest[1 : endQuote+1]
			}
		} else {
			f := strings.Fields(rest)
			if len(f) > 0 {
				fileTarget = f[0]
			}
		}
	}

	if strings.Contains(headerLine, ":mangle") {
		idx := strings.Index(headerLine, ":mangle")
		rest := strings.TrimSpace(headerLine[idx+7:])
		if strings.HasPrefix(rest, "(") {
			endParen := strings.Index(rest, ")")
			if endParen != -1 {
				filterStr := rest[1:endParen]
				for _, f := range strings.Split(filterStr, ",") {
					f = strings.TrimSpace(f)
					if f != "" {
						mangles = append(mangles, f)
					}
				}
			}
		}
	}

	return name, fileTarget, mangles
}

// WeaveMarkdown renders a PodDoc AST into GitHub-Flavored Markdown.
func WeaveMarkdown(doc *PodDoc) string {
	var sb strings.Builder

	for _, sec := range doc.Sections {
		switch sec.Type {
		case PodNodeHeading:
			sb.WriteString("\n" + strings.Repeat("#", sec.Level) + " " + formatPodInline(sec.Heading) + "\n\n")

		case PodNodeItem:
			sb.WriteString("- " + formatPodInline(strings.Join(sec.Content, " ")) + "\n")

		case PodNodeProse:
			proseText := strings.TrimSpace(strings.Join(sec.Content, "\n"))
			if proseText != "" {
				sb.WriteString(formatPodInline(proseText) + "\n\n")
			}

		case PodNodeChunk:
			if sec.Chunk != nil {
				targetInfo := ""
				if sec.Chunk.FileTarget != "" {
					targetInfo = fmt.Sprintf(" *(target: `%s`)*", sec.Chunk.FileTarget)
				}
				if len(sec.Chunk.Mangles) > 0 {
					targetInfo += fmt.Sprintf(" *[mangle: %s]*", strings.Join(sec.Chunk.Mangles, ", "))
				}
				sb.WriteString(fmt.Sprintf("\n**«%s»**%s:\n```raptor\n%s\n```\n\n",
					sec.Chunk.Name,
					targetInfo,
					strings.Join(sec.Chunk.Lines, "\n"),
				))
			}

		case PodNodeVerbatim:
			sb.WriteString("```raptor\n" + strings.Join(sec.Content, "\n") + "\n```\n\n")
		}
	}

	return strings.TrimSpace(sb.String()) + "\n"
}

func formatPodInline(text string) string {
	text = podFormatBRe.ReplaceAllString(text, "**$1**")
	text = podFormatIRe.ReplaceAllString(text, "*$1*")
	text = podFormatCRe.ReplaceAllString(text, "`$1`")
	text = podFormatLRe.ReplaceAllString(text, "[$1]($1)")
	return text
}

// Tangle extracts source files from the literate document by resolving <<chunk>> macros.
func Tangle(doc *PodDoc, targetFilter string) (map[string]string, error) {
	results := make(map[string]string)

	// Identify root chunks
	rootChunks := make([]*CodeChunk, 0)
	for _, chunk := range doc.Chunks {
		if targetFilter != "" {
			if chunk.Name == targetFilter || chunk.FileTarget == targetFilter {
				rootChunks = append(rootChunks, chunk)
			}
		} else {
			if chunk.FileTarget != "" || chunk.Name == "main" || len(doc.Chunks) == 1 {
				rootChunks = append(rootChunks, chunk)
			}
		}
	}

	if len(rootChunks) == 0 && len(doc.Chunks) > 0 {
		// Default to all chunks if no explicit file targets
		for _, chunk := range doc.Chunks {
			rootChunks = append(rootChunks, chunk)
		}
	}

	for _, root := range rootChunks {
		visited := make(map[string]bool)
		expanded, err := expandChunk(root, doc, visited, "")
		if err != nil {
			return nil, err
		}

		// Apply mangle pipeline
		if len(root.Mangles) > 0 {
			expanded = Mangle(expanded, root.Mangles)
		}

		targetKey := root.FileTarget
		if targetKey == "" {
			targetKey = root.Name + ".rp"
		}
		results[targetKey] = expanded
	}

	return results, nil
}

func expandChunk(chunk *CodeChunk, doc *PodDoc, visited map[string]bool, baseIndent string) (string, error) {
	if visited[chunk.Name] {
		return "", fmt.Errorf("circular chunk dependency detected at <<%s>>", chunk.Name)
	}
	visited[chunk.Name] = true
	defer func() { visited[chunk.Name] = false }()

	var lines []string
	for _, line := range chunk.Lines {
		indent := getLineIndent(line)
		matches := chunkMacroRe.FindAllStringSubmatch(line, -1)
		if len(matches) > 0 {
			for _, m := range matches {
				refName := m[2]
				if refName == "" {
					refName = m[3]
				}
				refChunk, exists := doc.Chunks[refName]
				if !exists {
					return "", fmt.Errorf("undefined code chunk <<%s>> referenced in <<%s>>", refName, chunk.Name)
				}

				subExpanded, err := expandChunk(refChunk, doc, visited, baseIndent+indent)
				if err != nil {
					return "", err
				}

				if len(refChunk.Mangles) > 0 {
					subExpanded = Mangle(subExpanded, refChunk.Mangles)
				}

				// Apply caller indentation to subchunk lines
				subLines := strings.Split(subExpanded, "\n")
				for i, sLine := range subLines {
					if i == 0 && strings.TrimSpace(line) == m[0] {
						subLines[i] = indent + sLine
					} else if sLine != "" {
						subLines[i] = indent + sLine
					}
				}
				expandedBlock := strings.Join(subLines, "\n")
				line = strings.Replace(line, m[0], expandedBlock, 1)
			}
			lines = append(lines, line)
		} else {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n"), nil
}

func getLineIndent(line string) string {
	var sb strings.Builder
	for _, r := range line {
		if r == ' ' || r == '\t' {
			sb.WriteRune(r)
		} else {
			break
		}
	}
	return sb.String()
}

// Mangle applies code transformations based on filter directives.
func Mangle(code string, filters []string) string {
	result := code
	for _, f := range filters {
		f = strings.TrimSpace(f)
		switch {
		case strings.HasPrefix(f, "indent("):
			nStr := strings.TrimPrefix(f, "indent(")
			nStr = strings.TrimSuffix(nStr, ")")
			if n, err := strconv.Atoi(nStr); err == nil && n > 0 {
				pad := strings.Repeat(" ", n)
				var outLines []string
				for _, line := range strings.Split(result, "\n") {
					if strings.TrimSpace(line) == "" {
						outLines = append(outLines, "")
					} else {
						outLines = append(outLines, pad+line)
					}
				}
				result = strings.Join(outLines, "\n")
			}

		case f == "strip_comments":
			var outLines []string
			for _, line := range strings.Split(result, "\n") {
				trimmed := strings.TrimSpace(line)
				if !strings.HasPrefix(trimmed, "#") {
					outLines = append(outLines, line)
				}
			}
			result = strings.Join(outLines, "\n")

		case strings.HasPrefix(f, "prefix("):
			p := strings.TrimPrefix(f, "prefix(")
			p = strings.TrimSuffix(p, ")")
			p = strings.Trim(p, "\"")
			result = p + result
		}
	}
	return result
}

// Stitch updates code chunks in a literate POD document from a map of updated source files.
func Stitch(podSource string, files map[string]string) (string, error) {
	doc, err := ParsePodDoc(podSource)
	if err != nil {
		return "", err
	}

	replacements := make(map[string][]string)

	for chunkName, chunk := range doc.Chunks {
		if content, ok := files[chunkName]; ok {
			replacements[chunkName] = strings.Split(content, "\n")
			continue
		}
		if chunk.FileTarget != "" {
			if content, ok := files[chunk.FileTarget]; ok {
				extracted := extractMarkedChunk(content, chunkName)
				if extracted != nil {
					replacements[chunkName] = extracted
				} else if !hasNestedChunks(chunk) {
					replacements[chunkName] = strings.Split(content, "\n")
				}
			}
		}
		for fPath, content := range files {
			if chunk.FileTarget != "" && filepath.Base(chunk.FileTarget) == filepath.Base(fPath) {
				extracted := extractMarkedChunk(content, chunkName)
				if extracted != nil {
					replacements[chunkName] = extracted
				} else if !hasNestedChunks(chunk) {
					replacements[chunkName] = strings.Split(content, "\n")
				}
			}
		}
	}

	lines := strings.Split(podSource, "\n")
	var outLines []string
	inChunk := false
	var activeChunkName string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "=") {
			if strings.HasPrefix(trimmed, "=chunk") || strings.HasPrefix(trimmed, "=code") || (strings.HasPrefix(trimmed, "=begin") && (strings.Contains(trimmed, "chunk") || strings.Contains(trimmed, "code"))) {
				name, _, _ := parseChunkHeader(trimmed)
				inChunk = true
				activeChunkName = name
				outLines = append(outLines, line)
				if newLines, ok := replacements[activeChunkName]; ok {
					outLines = append(outLines, newLines...)
				}
				continue
			}

			if inChunk && (strings.HasPrefix(trimmed, "=end") || trimmed == "=cut") {
				inChunk = false
				activeChunkName = ""
				outLines = append(outLines, line)
				continue
			}
		}

		if inChunk {
			if _, ok := replacements[activeChunkName]; ok {
				continue
			}
			outLines = append(outLines, line)
		} else {
			outLines = append(outLines, line)
		}
	}

	return strings.Join(outLines, "\n"), nil
}

func hasNestedChunks(chunk *CodeChunk) bool {
	for _, l := range chunk.Lines {
		if chunkMacroRe.MatchString(l) {
			return true
		}
	}
	return false
}

func extractMarkedChunk(content string, chunkName string) []string {
	lines := strings.Split(content, "\n")
	startMarker := fmt.Sprintf("<<<chunk:%s>>>", chunkName)
	altStartMarker := fmt.Sprintf("<<< %s >>>", chunkName)
	endMarker := fmt.Sprintf(">>>chunk:%s>>>", chunkName)
	altEndMarker := fmt.Sprintf(">>> %s >>>", chunkName)

	var chunkLines []string
	capturing := false

	for _, line := range lines {
		if strings.Contains(line, startMarker) || strings.Contains(line, altStartMarker) {
			capturing = true
			continue
		}
		if capturing && (strings.Contains(line, endMarker) || strings.Contains(line, altEndMarker)) {
			capturing = false
			break
		}
		if capturing {
			chunkLines = append(chunkLines, line)
		}
	}

	if len(chunkLines) > 0 {
		return chunkLines
	}
	return nil
}

// RegisterPodLitBuiltins exposes pod_weave, pod_tangle, and pod_stitch to the Raptor runtime.
func (in *Interp) registerPodLitBuiltins() {
	in.Builtins["pod_weave"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("pod_weave requires pod source text")
		}
		doc, err := ParsePodDoc(args[0].String())
		if err != nil {
			return nil, fmt.Errorf("pod parse failed: %w", err)
		}
		return StringValue(WeaveMarkdown(doc)), nil
	}

	in.Builtins["pod_tangle"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("pod_tangle requires pod source text")
		}
		target := ""
		if len(args) >= 2 && args[1].Type != ValNil {
			target = args[1].String()
		}
		doc, err := ParsePodDoc(args[0].String())
		if err != nil {
			return nil, fmt.Errorf("pod parse failed: %w", err)
		}
		tangled, err := Tangle(doc, target)
		if err != nil {
			return nil, fmt.Errorf("tangle failed: %w", err)
		}
		res := make(map[string]*Value)
		for k, v := range tangled {
			res[k] = StringValue(v)
		}
		return HashValue(res), nil
	}

	in.Builtins["pod_stitch"] = func(in *Interp, args []*Value) (*Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("pod_stitch requires pod source text and files map")
		}
		podText := args[0].String()
		filesMap := make(map[string]string)

		if args[1].Type == ValHash && args[1].HashVal != nil {
			for k, v := range args[1].HashVal {
				filesMap[k] = v.String()
			}
		} else if args[1].Type == ValString {
			filesMap["main"] = args[1].StrVal
		}

		updatedPod, err := Stitch(podText, filesMap)
		if err != nil {
			return nil, fmt.Errorf("stitch failed: %w", err)
		}
		return StringValue(updatedPod), nil
	}
}
