# Raptor Testing, TAP Protocol & Verification System

Raptor features a built-in verification framework supporting Perl5 `Test::More` TAP output, a `raptor test` harness (like `prove`), zero-overhead inline `TEST` blocks, Design-by-Contract (`pre`, `post`, `invariant`), and Property-Based QuickCheck fuzzing.

---

## 1. TAP (Test Anything Protocol) Testing

Write test scripts with `.t` extension:

```perl
plan(6);

# Basic boolean assertion
ok(1 + 1 == 2, "Addition holds");

# Equality with automatic diffs on failure
is(2 * 3, 6, "Multiplication produces 6");
isnt("alpha", "beta", "Strings are different");

# Deep structural equality
is_deeply([1, 2, {:a => 10}], [1, 2, {:a => 10}], "Deep structure matches");

# Pattern matching
like("raptor_1.0.0", "^raptor_", "Prefix matches");
unlike("hello world", "[0-9]", "No numbers found");

done_testing();
```

Run test suites using the `raptor test` CLI runner:
```powershell
raptor test t/
```

---

## 2. Zero-Overhead Inline Tests (`TEST`)

Embed tests directly alongside production code. During standard execution, `TEST` blocks are completely skipped. When executed via `raptor test` or `--test`, they run automatically:

```perl
sub calculate_tax($subtotal, $rate) {
    return $subtotal * $rate;
}

# Zero runtime overhead in production
TEST "calculate_tax calculations", sub () {
    is(calculate_tax(100.0, 0.05), 5.0, "5% tax on 100 is 5.0");
    is(calculate_tax(200.0, 0.10), 20.0, "10% tax on 200 is 20.0");
};
```

---

## 3. Design-by-Contract (`pre`, `post`, `invariant`)

Enforce preconditions and postconditions dynamically:

```perl
sub deposit($account, $amount) {
    pre($amount > 0, "Deposit amount must be positive");
    
    $account<balance> = $account<balance> + $amount;
    
    post($account<balance> >= $amount, "Balance must reflect deposit");
    return $account<balance>;
}
```

---

## 4. Property-Based QuickCheck Fuzzing

Assert mathematical and algorithmic invariants across 100 randomized input trials:

```perl
# Verify commutativity of addition across random integers
property "Addition Commutativity", sub ($a, $b) {
    return ($a + $b) == ($b + $a);
};

# Verify string reverse involution
property "String Reversal Involution", sub ($s) {
    return reverse(reverse($s)) eq $s;
};
```
