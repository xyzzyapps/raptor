# Test suite: Operators & Basic Language Semantics
plan(14);

# Defined-or
my $val = Nil // "default_val";
is($val, "default_val", "defined-or returns default on nil");

my $assigned = "existing";
$assigned //= "fallback";
is($assigned, "existing", "defined-or assign preserves defined value");

# Exponentiation
is(2 ** 10, 1024, "2**10 equals 1024");

# Ternary
my $tern1 = 10 > 5 ?? "yes" !! "no";
is($tern1, "yes", "true ternary branch");
my $tern2 = 10 < 5 ?? "yes" !! "no";
is($tern2, "no", "false ternary branch");

# Bitwise
is(0x0F +| 0xF0, 255, "bitwise OR");
is(0xFF +& 0x0F, 15, "bitwise AND");
is(0xFF +^ 0x0F, 240, "bitwise XOR");

# Repetition
is("A" x 5, "AAAAA", "string repetition x");
is_deeply([1, 2] xx 2, [1, 2, 1, 2], "list repetition xx");

# Divisibility
ok(100 %% 2, "100 is divisible by 2");
ok(!(101 %% 2), "101 is not divisible by 2");

# Min / Max
is(10 min 20, 10, "min operator");
is(10 max 20, 20, "max operator");

done_testing();
