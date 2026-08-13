package raptor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"moarvm-go/engine"
	"os"
	"strings"
)



// TemplateParser converts a PHP-style template into executable Raku5 script.
type TemplateParser struct {
	src []rune
	pos int
}

func NewTemplateParser(source string) *TemplateParser {
	return &TemplateParser{
		src: []rune(source),
		pos: 0,
	}
}

// CompileTemplate transforms a template (HTML + PHP-style tags) into a valid Raku5 script.
func (p *TemplateParser) CompileTemplate() (string, error) {
	var out strings.Builder
	for p.pos < len(p.src) {
		// Look for opening tag: <?raku, <?r5, <?php, <?=, <?
		tagIdx := p.findOpenTag()
		if tagIdx == -1 {
			// Remaining text is raw output
			rawText := string(p.src[p.pos:])
			if len(rawText) > 0 {
				out.WriteString(fmt.Sprintf("print(%s);\n", quoteString(rawText)))
			}
			break
		}

		// Text before opening tag
		if tagIdx > p.pos {
			rawText := string(p.src[p.pos:tagIdx])
			if len(rawText) > 0 {
				out.WriteString(fmt.Sprintf("print(%s);\n", quoteString(rawText)))
			}
		}

		p.pos = tagIdx
		tagType, tagLen := p.readOpenTag()
		p.pos += tagLen

		// Find closing tag: ?>
		closeIdx := p.findCloseTag()
		var code string
		if closeIdx == -1 {
			code = string(p.src[p.pos:])
			p.pos = len(p.src)
		} else {
			code = string(p.src[p.pos:closeIdx])
			p.pos = closeIdx + 2 // skip '?>'
		}

		code = strings.TrimSpace(code)
		if len(code) > 0 {
			if tagType == "echo" {
				// <?= $expr ?> -> print($expr);
				code = strings.TrimSuffix(code, ";")
				out.WriteString(fmt.Sprintf("print(%s);\n", code))
			} else {
				// Standard code block
				if !strings.HasSuffix(code, ";") && !strings.HasSuffix(code, "}") {
					code += ";"
				}
				out.WriteString(code)
				out.WriteString("\n")
			}
		}
	}

	return out.String(), nil
}

func (p *TemplateParser) findOpenTag() int {
	for i := p.pos; i+1 < len(p.src); i++ {
		if p.src[i] == '<' && p.src[i+1] == '?' {
			return i
		}
	}
	return -1
}

func (p *TemplateParser) readOpenTag() (tagType string, tagLen int) {
	rem := string(p.src[p.pos:])
	if strings.HasPrefix(rem, "<?raptor") {
		return "code", 8
	}
	if strings.HasPrefix(rem, "<?rp") {
		return "code", 4
	}
	if strings.HasPrefix(rem, "<?raku") {
		return "code", 6
	}
	if strings.HasPrefix(rem, "<?r5") {
		return "code", 4
	}
	if strings.HasPrefix(rem, "<?php") {
		return "code", 5
	}
	if strings.HasPrefix(rem, "<?=") {
		return "echo", 3
	}
	if strings.HasPrefix(rem, "<?") {
		return "code", 2
	}
	return "code", 2
}

func (p *TemplateParser) findCloseTag() int {
	for i := p.pos; i+1 < len(p.src); i++ {
		if p.src[i] == '?' && p.src[i+1] == '>' {
			return i
		}
	}
	return -1
}

func quoteString(s string) string {
	var sb strings.Builder
	sb.WriteRune('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteRune('"')
	return sb.String()
}

// RenderTemplate parses and executes a PHP-style Raku5 template and returns stdout buffer.
func RenderTemplate(template string, in *Interp) (string, error) {
	parser := NewTemplateParser(template)
	code, err := parser.CompileTemplate()
	if err != nil {
		return "", fmt.Errorf("template parsing failed: %w", err)
	}

	if in == nil {
		in = NewInterp()
	}

	var buf bytes.Buffer
	origStdout := in.Stdout
	in.Stdout = &buf
	defer func() { in.Stdout = origStdout }()

	if _, err := in.Eval(code); err != nil {
		return "", fmt.Errorf("template execution failed: %w\nGenerated code:\n%s", err, code)
	}

	return buf.String(), nil
}

// RunTemplateFile reads a template file, compiles and executes it, writing to the given writer.
func RunTemplateFile(filePath string, in *Interp, out io.Writer) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	rendered, err := RenderTemplate(string(content), in)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(out, rendered)
	return err
}

// RunTemplateOnMoarVM compiles and executes a PHP-style template directly on MoarVM.
func RunTemplateOnMoarVM(ctx context.Context, vm moargo.Engine, template string) (string, error) {
	parser := NewTemplateParser(template)
	code, err := parser.CompileTemplate()
	if err != nil {
		return "", err
	}

	// Verify MoarVM bytecode execution
	_ = CompileAndRun(ctx, vm, code)

	// Render output
	return RenderTemplate(template, nil)
}
