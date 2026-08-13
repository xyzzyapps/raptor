# Test suite: Verification, Contracts & Property-Based Fuzzing
plan(4);

# 1. Design-by-Contract: pre & post conditions
sub safe_divide($a, $b) {
    pre($b != 0, "divisor cannot be zero");
    my $res = $a / $b;
    post($res * $b == $a, "inverse multiplication holds");
    return $res;
}

is(safe_divide(100, 4), 25, "safe_divide holds contracts for valid inputs");

# 2. Zero-overhead inline tests
TEST "inline mathematical operations", sub () {
    is(10 + 20, 30, "inline test sum");
    is(5 * 5, 25, "inline test product");
};

# 3. Property-Based QuickCheck Testing (100 randomized trials)
property "addition commutativity", sub ($a, $b) {
    return ($a + $b) == ($b + $a);
};

property "multiplication by zero", sub ($n) {
    return ($n * 0) == 0;
};

done_testing();
