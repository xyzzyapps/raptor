# Grep/map/sum in Perl 5
my $sum = 0;
for my $i (1..200000) {
    if ($i % 3 == 0) {
        $sum += ($i + 1);
    }
}
print "$sum\n";
