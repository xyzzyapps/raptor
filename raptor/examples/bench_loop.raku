my $s = 0;
for 1..100000 -> $i {
    $s = $s + $i;
}
say "Raku5 Loop Result: $s";
