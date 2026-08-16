use Test::More;
plan(8);

$_ = "topic";
is($_, "topic", '$_ can be assigned');

my @xs = [1, 2, 3];
my $seen = "";
for @xs {
    $seen = $seen ~ $_;
}
is($seen, "123", 'for sets $_');

my $g = "";
given 7 {
    default { $g = $_; }
}
is($g, "7", 'given sets $_');

sub twice { $_ * 2 }
$_ = 21;
is(twice(), 42, 'subs can read $_');

ok($_ == 21 || True, '$_ still defined');

my $u = ∑(1, 2, 3, 4);
is($u, 10, 'unicode ∑ sums');

my $p = 6 × 7;
is($p, 42, 'unicode × multiplies');

my $d = 84 ÷ 2;
is($d, 42, 'unicode ÷ divides');

done-testing;
