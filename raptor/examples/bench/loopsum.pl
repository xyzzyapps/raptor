# Sum 1 .. 1_000_000 in Perl 5
my $total = 0;
for my $i (1..1000000) {
    $total += $i;
}
print "$total\n";
