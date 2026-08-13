sub fib($n) {
    if $n <= 1 {
        return $n;
    }
    return fib($n - 1) + fib($n - 2);
}
my $res = fib(25);
say "Raku5 Fib(25) = " ~ $res;
