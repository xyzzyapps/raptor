plan(6);

# 1. Basic backtick shell command execution
my $out = `echo 42`;
is($out, "42", "backtick echo captures stdout output");

# 2. Variable interpolation in backticks
my $msg = "hello_raptor";
my $interp_out = `echo $msg`;
is($interp_out, "hello_raptor", "interpolated variable in backtick command");

# 3. Success exit code in $?
is($?, 0, "\$? is 0 after successful command");

# 4. Alternative qx operator
my $qx_out = qx{echo qx_works};
is($qx_out, "qx_works", "qx{} syntax captures output");

# 5. Pipeline / expression execution
my $eval_out = `echo test_eval`;
is($eval_out, "test_eval", "backtick command executed successfully");

# 6. Punctuation variable $! is nil on success
is($!, Nil, "\$! is Nil on successful command");

done_testing();
