plan(4);

# 1. Global AUTOLOAD subroutine
my $captured_func = "";
my $captured_args = [];

sub AUTOLOAD($a, $b) {
    $captured_func = $AUTOLOAD;
    $captured_args = [$a, $b];
    return "AUTOLOAD_HANDLED";
}

my $res = compute_magic_sum(100, 200);
is($res, "AUTOLOAD_HANDLED", "AUTOLOAD returned handler result");
is($captured_func, "main::compute_magic_sum", "\$AUTOLOAD contains missing function name");
is($captured_args->[0], 100, "AUTOLOAD received first argument");
is($captured_args->[1], 200, "AUTOLOAD received second argument");

done_testing();
