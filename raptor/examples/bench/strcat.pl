# 50_000 concatenations in Perl 5
my $s = "";
for (1..50000) {
    $s .= "x";
}
print length($s), "\n";
