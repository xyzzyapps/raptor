# 100_000 hash increments in Perl 5
my %counts;
for my $i (1..100000) {
    $counts{$i % 1000}++;
}
my $elems = scalar(keys %counts);
print "$elems $counts{0}\n";
