plan(8);

my $ctx = ggml_init(65536);
ok($ctx, "ggml_init");

my $x = ggml_new_tensor_1d($ctx, $GGML_TYPE_F32, 2);
ggml_set_f32_1d($x, 0, 1.0);
ggml_set_f32_1d($x, 1, -3.0);
is(ggml_nelements($x), 2, "nelements");

my $y = ggml_relu($ctx, $x);
my $gf = ggml_new_graph($ctx);
ok(ggml_build_forward_expand($gf, $y), "build_forward_expand");
ok(ggml_graph_compute_with_ctx($ctx, $gf, 1), "graph_compute");

is(ggml_get_f32_1d($y, 0), 1, "relu keeps positive");
is(ggml_get_f32_1d($y, 1), 0, "relu zeros negative");

my $m = llm_tiny_load();
ok($m, "llm_tiny_load");
my $gen = llm_tiny_generate($m, "the ", 8, 0.3);
ok($gen, "llm_tiny_generate");

done_testing();
