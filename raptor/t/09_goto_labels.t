plan(3);

# 1. Forward goto jump
my $reached = False;
goto SKIP_POINT;
$reached = True;

SKIP_POINT:
is($reached, False, "goto skips over bypassed statements");

# 2. Backward goto loop
my $count = 0;

LOOP_START:
$count = $count + 1;
if $count < 5 {
    goto LOOP_START;
}

is($count, 5, "goto backward jump implements loop");

# 3. Goto &sub tail call
sub target_sub() {
    return "called target";
}

sub forwarder_sub() {
    goto &target_sub;
}

is(forwarder_sub(), "called target", "goto &sub invokes target subroutine");

done_testing();
