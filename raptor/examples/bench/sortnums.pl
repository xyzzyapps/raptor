# Sort 50_000 pseudo-random ints in Perl 5
my @nums;
for my $i (1..50000) {
    push @nums, int(($i * 2654435761) % 100000);
}
my @sorted = sort { $a <=> $b } @nums;
print "$sorted[0] $sorted[-1] ", scalar(@sorted), "\n";
