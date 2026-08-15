plan(6);

# 1. Default GoRegexp engine
is(regex_engine(), "GoRegexp", "initial regex engine is GoRegexp");

my $text = "Raptor post-LLM language runtime 2026";
ok($text =~ "post-LLM", "=~ matches substring with GoRegexp");
ok($text !~ "python", "!~ asserts non-matching substring");

# 2. Switch to samre regex engine
is(regex_engine("samre"), "samre", "switched active regex engine to samre");

# 3. Pattern matching with samre backend
ok($text =~ "runtime", "=~ matches pattern with samre backend");
ok($text !~ "javascript", "!~ asserts non-matching pattern with samre backend");

done_testing();
