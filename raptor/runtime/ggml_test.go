package raptor

import (
	"math"
	"testing"
)

func TestGGMLTensorAPI(t *testing.T) {
	in := NewInterp()
	code := `
my $ctx = ggml_init(1024 * 1024);
my $a = ggml_new_tensor_2d($ctx, $GGML_TYPE_F32, 2, 2);
my $b = ggml_new_tensor_2d($ctx, $GGML_TYPE_F32, 2, 2);
ggml_set_name($a, "A");
ggml_set_f32_nd($a, 0, 0, 1.0);
ggml_set_f32_nd($a, 1, 0, 2.0);
ggml_set_f32_nd($a, 0, 1, 3.0);
ggml_set_f32_nd($a, 1, 1, 4.0);
ggml_set_f32_nd($b, 0, 0, 5.0);
ggml_set_f32_nd($b, 1, 0, 6.0);
ggml_set_f32_nd($b, 0, 1, 7.0);
ggml_set_f32_nd($b, 1, 1, 8.0);

my $sum = ggml_add($ctx, $a, $b);
my $prod = ggml_mul_mat($ctx, $a, $b);
my $rel = ggml_relu($ctx, $a);
my $gf = ggml_new_graph($ctx);
ggml_build_forward_expand($gf, $sum);
ggml_build_forward_expand($gf, $prod);
ggml_build_forward_expand($gf, $rel);
my $ok = ggml_graph_compute_with_ctx($ctx, $gf, 1);

my $tuple = [$ok, ggml_get_name($a), ggml_nelements($a), ggml_get_f32_nd($sum, 0, 0), ggml_get_f32_nd($prod, 0, 0), ggml_backend(), ggml_n_dims($a)];
ggml_free($ctx);
$tuple;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("ggml eval: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 7 {
		t.Fatalf("expected 7-tuple, got %+v", val)
	}
	if !val.ArrayVal[0].IsTrue() {
		t.Errorf("graph compute failed")
	}
	if val.ArrayVal[1].String() != "A" {
		t.Errorf("name: %q", val.ArrayVal[1].String())
	}
	if val.ArrayVal[2].IntVal != 4 {
		t.Errorf("nelements: %v", val.ArrayVal[2])
	}
	if math.Abs(ggmlAsFloat(val.ArrayVal[3])-6.0) > 1e-5 {
		t.Errorf("add 1+5: got %v", val.ArrayVal[3])
	}
	// mul_mat A^T * B: A col0 = [1,2], B col0 = [5,6] => 1*5+2*6 = 17
	if math.Abs(ggmlAsFloat(val.ArrayVal[4])-17.0) > 1e-4 {
		t.Errorf("mul_mat: got %v want 17", val.ArrayVal[4])
	}
	if val.ArrayVal[5].String() == "" {
		t.Errorf("empty backend")
	}
	if val.ArrayVal[6].IntVal != 2 {
		t.Errorf("n_dims: %v", val.ArrayVal[6])
	}
}

func TestGGMLActivationsAndSoftmax(t *testing.T) {
	in := NewInterp()
	code := `
my $ctx = ggml_init(65536);
my $x = ggml_new_tensor_1d($ctx, 0, 3);
ggml_set_f32_1d($x, 0, -1.0);
ggml_set_f32_1d($x, 1, 0.0);
ggml_set_f32_1d($x, 2, 2.0);
my $r = ggml_relu($ctx, $x);
my $s = ggml_soft_max($ctx, $x);
my $g = ggml_gelu($ctx, $x);
my $gf = ggml_new_graph($ctx);
ggml_build_forward_expand($gf, $r);
ggml_build_forward_expand($gf, $s);
ggml_build_forward_expand($gf, $g);
ggml_graph_compute($ctx, $gf);
[ggml_get_f32_1d($r, 0), ggml_get_f32_1d($r, 2), ggml_get_f32_1d($s, 0) + ggml_get_f32_1d($s, 1) + ggml_get_f32_1d($s, 2)];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("bad result %+v", val)
	}
	if math.Abs(ggmlAsFloat(val.ArrayVal[0])-0) > 1e-6 {
		t.Errorf("relu(-1) = %v", val.ArrayVal[0])
	}
	if math.Abs(ggmlAsFloat(val.ArrayVal[1])-2) > 1e-6 {
		t.Errorf("relu(2) = %v", val.ArrayVal[1])
	}
	if math.Abs(ggmlAsFloat(val.ArrayVal[2])-1) > 1e-4 {
		t.Errorf("softmax sum = %v", val.ArrayVal[2])
	}
}

func TestGGMLScaleAndGetData(t *testing.T) {
	in := NewInterp()
	code := `
my $ctx = ggml_init(4096);
my $x = ggml_new_tensor_1d($ctx, 0, 2);
ggml_set_data($x, [1.5, -2.0]);
my $y = ggml_scale($ctx, $x, 2.0);
my $gf = ggml_new_graph($ctx);
ggml_build_forward_expand($gf, $y);
ggml_graph_compute_with_ctx($ctx, $gf, 1);
my $out = [ggml_get_f32_1d($y, 0), ggml_get_f32_1d($y, 1)];
ggml_free($ctx);
$out;
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("bad %+v", val)
	}
	if math.Abs(ggmlAsFloat(val.ArrayVal[0])-3.0) > 1e-5 {
		t.Errorf("scale 0: %v", val.ArrayVal[0])
	}
	if math.Abs(ggmlAsFloat(val.ArrayVal[1])+4.0) > 1e-5 {
		t.Errorf("scale 1: %v", val.ArrayVal[1])
	}
}
