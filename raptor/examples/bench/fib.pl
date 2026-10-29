# Naive recursive fib(29) in Perl 5
sub fib {
    my ($n) = @_;
    return $n if $n < 2;
    return fib($n - 1) + fib($n - 2);
}
print fib(29), "\n";
