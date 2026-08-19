plan 7;

# 1. PRE and POST contracts
sub safe_div($a, $b) {
    PRE { $b != 0 }, "denominator cannot be zero";
    my $res = $a div $b;
    POST { $res >= 0 }, "result must be non-negative";
    return $res;
}

is safe_div(10, 2), 5, "PRE and POST succeed on valid division";

# 2. INVARIANT contract
sub account_deposit($balance, $amount) {
    INVARIANT { $balance >= 0 }, "balance cannot be negative";
    PRE { $amount > 0 }, "deposit amount must be positive";
    my $new_bal = $balance + $amount;
    INVARIANT { $new_bal >= 0 }, "new balance invariant";
    return $new_bal;
}

is account_deposit(100, 50), 150, "INVARIANT succeeds on valid state";

# 3. CHECK and ASSERT assertions
CHECK 10 > 5, "10 is greater than 5";
ok True, "CHECK passes on true condition";
ASSERT 20 == 20, "20 equals 20";
ok True, "ASSERT passes on equality";

# 4. SUBTEST nested suite
SUBTEST "Uppercase Nested Subtest" {
    plan 2;
    ok True, "subtest assertion 1";
    is 1 + 1, 2, "subtest assertion 2";
}

# 5. PROPERTY QuickCheck fuzzing
PROPERTY "addition commutativity" ($a, $b) {
    return ($a + $b) == ($b + $a);
}

PROPERTY "identity of zero" ($a) {
    return ($a + 0) == $a;
}

done_testing;
