# 1M string eq/lt comparisons in Perl 5
my $a = "delta";
my $b = "delt" . "a";
my $c = 0;
for (1..1000000) {
    $c++ if $a eq $b;
    $c-- if $a lt $b;
}
print "$c\n";
