sub fib {
    my ($n) = @_;
    return $n if $n <= 1;
    return fib($n - 1) + fib($n - 2);
}
my $res = fib(25);
print "Perl5 Fib(25) = $res\n";
