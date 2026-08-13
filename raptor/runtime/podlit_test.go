package raptor

import (
	"strings"
	"testing"
)

func TestPodWeaveToMarkdown(t *testing.T) {
	podSrc := `
=pod

=head1 Math Utilities

This module contains core math functions.

=item * Fast addition
=item * Precise multiplication

=chunk <add-sub> :file "lib/Add.rp"
sub add($a, $b) {
    return $a + $b;
}
=end chunk

=cut
`
	doc, err := ParsePodDoc(podSrc)
	if err != nil {
		t.Fatalf("ParsePodDoc failed: %v", err)
	}

	md := WeaveMarkdown(doc)
	if !strings.Contains(md, "# Math Utilities") {
		t.Errorf("Expected '# Math Utilities' in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "**«add-sub»** *(target: `lib/Add.rp`)*:") {
		t.Errorf("Expected chunk header in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "```raptor\nsub add($a, $b) {") {
		t.Errorf("Expected code fence in markdown, got:\n%s", md)
	}
}

func TestPodTangleRecursiveChunks(t *testing.T) {
	podSrc := `
=pod

=head1 Particle Physics

=chunk <vector-struct>
struct Vec2 {
    num32 $x;
    num32 $y;
}
=end chunk

=chunk <bounce-logic>
if $p.x <= 0 { $p.vx = -$p.vx; }
=end chunk

=chunk <main-app> :file "bin/app.rp"
# Generated
<<vector-struct>>

sub update($p) {
    <<bounce-logic>>
}
=end chunk

=cut
`
	doc, err := ParsePodDoc(podSrc)
	if err != nil {
		t.Fatalf("ParsePodDoc failed: %v", err)
	}

	files, err := Tangle(doc, "")
	if err != nil {
		t.Fatalf("Tangle failed: %v", err)
	}

	appCode, ok := files["bin/app.rp"]
	if !ok {
		t.Fatalf("Expected 'bin/app.rp' in tangled files, got: %v", files)
	}

	if !strings.Contains(appCode, "struct Vec2 {") {
		t.Errorf("Expected expanded vector-struct in tangled appCode, got:\n%s", appCode)
	}
	if !strings.Contains(appCode, "    if $p.x <= 0 { $p.vx = -$p.vx; }") {
		t.Errorf("Expected indented bounce-logic in tangled appCode, got:\n%s", appCode)
	}
}

func TestPodTangleCycleDetection(t *testing.T) {
	podSrc := `
=pod

=chunk <alpha>
<<beta>>
=end chunk

=chunk <beta>
<<alpha>>
=end chunk

=chunk <root> :file "root.rp"
<<alpha>>
=end chunk

=cut
`
	doc, err := ParsePodDoc(podSrc)
	if err != nil {
		t.Fatalf("ParsePodDoc failed: %v", err)
	}

	_, err = Tangle(doc, "")
	if err == nil {
		t.Fatalf("Expected cycle detection error, got nil")
	}
	if !strings.Contains(err.Error(), "circular chunk dependency") {
		t.Errorf("Expected circular chunk error message, got: %v", err)
	}
}

func TestPodMangleFilters(t *testing.T) {
	code := "# header comment\nline 1\nline 2"
	mangled := Mangle(code, []string{"strip_comments", "indent(4)"})
	expected := "    line 1\n    line 2"
	if mangled != expected {
		t.Errorf("Expected %q, got %q", expected, mangled)
	}
}

func TestPodRuntimeBuiltins(t *testing.T) {
	in := NewInterp()
	script := `
		my $pod = "=pod\n\n=head1 Hello\n\n=chunk <init> :file \"app.rp\"\nsay 42;\n=end chunk\n\n=cut";
		my $md = pod_weave($pod);
		my %tangled = pod_tangle($pod);
		my $app = %tangled{"app.rp"};
		[$md, $app];
	`
	val, err := in.Eval(script)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("Expected 2-element array, got %v", val)
	}

	mdStr := val.ArrayVal[0].String()
	appStr := val.ArrayVal[1].String()

	if !strings.Contains(mdStr, "# Hello") {
		t.Errorf("Expected '# Hello' in woven markdown, got: %s", mdStr)
	}
	if !strings.Contains(appStr, "say 42;") {
		t.Errorf("Expected 'say 42;' in tangled code, got: %s", appStr)
	}
}

func TestPodStitchRoundTrip(t *testing.T) {
	originalPod := `=pod

=head1 Math Module

=chunk <add-func> :file "lib/Add.rp"
sub add($a, $b) {
    return $a + $b;
}
=end chunk

=cut`

	modifiedFiles := map[string]string{
		"lib/Add.rp": "sub add($a, $b) {\n    # updated implementation\n    return ($a + $b) * 2;\n}",
	}

	updatedPod, err := Stitch(originalPod, modifiedFiles)
	if err != nil {
		t.Fatalf("Stitch failed: %v", err)
	}

	if !strings.Contains(updatedPod, "=head1 Math Module") {
		t.Errorf("Expected preserved header, got:\n%s", updatedPod)
	}
	if !strings.Contains(updatedPod, "# updated implementation") {
		t.Errorf("Expected updated chunk body in stitched POD, got:\n%s", updatedPod)
	}
	if !strings.Contains(updatedPod, "return ($a + $b) * 2;") {
		t.Errorf("Expected updated logic in stitched POD, got:\n%s", updatedPod)
	}
}
