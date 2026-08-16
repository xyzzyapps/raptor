package raptor

import (
	"strings"
	"testing"
)

func TestTinyLLMGenerate(t *testing.T) {
	in := NewInterp()
	code := `
my $m = llm_tiny_load();
my $out = llm_tiny_generate($m, "raptor is ", 32, 0.4);
my @logits = llm_tiny_logits($m, "raptor is ");
[llm_tiny_backend(), $out, @logits.elems(), llm_tiny_vocab()];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 4 {
		t.Fatalf("bad result %+v", val)
	}
	backend := val.ArrayVal[0].String()
	if backend != "cpu" && backend != "webgpu" {
		t.Errorf("backend %q", backend)
	}
	out := val.ArrayVal[1].String()
	if !strings.HasPrefix(out, "raptor is ") {
		t.Errorf("generate should keep prompt, got %q", out)
	}
	if len(out) <= len("raptor is ") {
		t.Errorf("expected generated continuation, got %q", out)
	}
	// Greedy (temp 0) should follow the corpus trigram after "raptor is ".
	codeG := `llm_tiny_generate(llm_tiny_load(), "raptor is ", 12, 0.0);`
	gval, gerr := in.Eval(codeG)
	if gerr != nil {
		t.Fatalf("greedy: %v", gerr)
	}
	got := gval.String()
	if !strings.HasPrefix(got, "raptor is a") {
		t.Errorf("greedy continuation, got %q", got)
	}
	if val.ArrayVal[2].IntVal != int64(len(tinyLMVocabSrc)) {
		t.Errorf("logits elems %v want %d", val.ArrayVal[2], len(tinyLMVocabSrc))
	}
	if val.ArrayVal[3].String() != tinyLMVocabSrc {
		t.Errorf("vocab mismatch")
	}
}

func TestTinyLLMSample(t *testing.T) {
	in := NewInterp()
	code := `
my $m = llm_tiny_load();
my @logits = llm_tiny_logits($m, "the ");
my $ch = llm_tiny_sample(@logits, 0.2);
$ch;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	s := val.String()
	if len(s) != 1 {
		t.Fatalf("sample returned %q", s)
	}
	if !strings.Contains(tinyLMVocabSrc, s) {
		t.Errorf("char %q not in vocab", s)
	}
}
