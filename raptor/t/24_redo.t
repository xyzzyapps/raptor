plan(6);

# while: redo restarts the body without re-checking the condition
my $hits = 0;
my $n = 0;
while $n < 1 {
    $hits = $hits + 1;
    if $hits == 1 {
        redo;
    }
    $n = $n + 1;
}
is($hits, 2, "while redo repeats the current body once");

# for: redo keeps the same iterator value
my $sum = 0;
my $extra = 1;
for 1..3 {
    $sum = $sum + $_;
    if $extra && $_ == 2 {
        $extra = 0;
        redo;
    }
}
is($sum, 8, "for redo reuses the current item (1+2+2+3)");

# C-style loop: redo skips the step
my $i = 0;
my $steps = 0;
loop (; $i < 3; $i = $i + 1) {
    $steps = $steps + 1;
    if $steps == 1 {
        redo;
    }
}
is($steps, 4, "loop redo skips the increment on the first pass");
is($i, 3, "loop increment still runs after redo");

# postfix if modifier
my $p = 0;
my $q = 0;
while $p < 1 {
    $p = $p + 1;
    $q = $q + 1;
    redo if $q == 1;
}
is($q, 2, "redo if modifier restarts the while body");

# last still exits the loop
my $k = 0;
for 1..5 {
    $k = $k + 1;
    if $_ == 2 {
        last;
    }
}
is($k, 2, "last still exits a loop that can redo");

done_testing();
