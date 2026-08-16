plan(6);

$_ = "topic";
is($_, "topic", '$_ can be assigned');

my $g = "";
given 7 {
    default { $g = $_; }
}
is($g, "7", 'given sets $_');

sub twice { $_ * 2 }
$_ = 21;
is(twice(), 42, 'subs can read $_');

ok(True, '$_ still defined');

my $p = 6 × 7;
is($p, 42, 'unicode × multiplies');

my $d = 84 ÷ 2;
is($d, 42, 'unicode ÷ divides');

done_testing();
