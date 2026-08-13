# Test suite: Subsets & Predicate Dispatching
plan(8);

subset Negative where { $_ < 0 };
subset Zero where { $_ == 0 };
subset Positive where { $_ > 0 };
subset Even where { $_ % 2 == 0 };

# Smart matching against subsets
ok(-5 ~~ Negative, "-5 is Negative");
ok(0 ~~ Zero, "0 is Zero");
ok(42 ~~ Positive, "42 is Positive");
ok(100 ~~ Even, "100 is Even");

# Predicate dispatching on multi subs
multi sub classify(Negative $n) { return "neg"; }
multi sub classify(Zero $n)     { return "zero"; }
multi sub classify(Positive $n) { return "pos"; }

is(classify(-10), "neg", "multi sub dispatched to Negative");
is(classify(0), "zero", "multi sub dispatched to Zero");
is(classify(99), "pos", "multi sub dispatched to Positive");

# Multi sub with inline where guard
multi sub fib($n where { $n <= 1 }) { return $n; }
multi sub fib($n where { $n > 1 })  { return fib($n - 1) + fib($n - 2); }

is(fib(7), 13, "fibonacci(7) via predicate dispatch");

done_testing();
