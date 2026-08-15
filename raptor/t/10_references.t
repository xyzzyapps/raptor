plan(10);

# 1. Scalar reference and dereference
my $val = 100;
my $sref = \$val;
is(ref($sref), "SCALAR", "ref(\\$val) is SCALAR");
is(is_ref($sref), True, "is_ref is True for reference");
is($$sref, 100, "dereferencing \\$val gives 100");

# 2. Scalar ref mutation
$$sref = 250;
is($val, 250, "mutating \\$$sref updates original variable");

# 3. Array reference and arrow indexing
my @nums = [10, 20, 30];
my $aref = \@nums;
is(ref($aref), "ARRAY", "ref(\\@nums) is ARRAY");
is($aref->[1], 20, "arrow dereference $aref->[1] gives 20");

$aref->[1] = 99;
is($aref->[1], 99, "arrow assignment $aref->[1] updates element");

# 4. Hash reference and arrow indexing
my %config = { "host" => "localhost", "port" => 8080 };
my $href = \%config;
is(ref($href), "HASH", "ref(\\%config) is HASH");
is($href->{"host"}, "localhost", "arrow dereference $href->{'host'} gives localhost");

# 5. Code reference and arrow call
sub add($a, $b) { return $a + $b; }
my $cref = \&add;
is(ref($cref), "CODE", "ref(\\&add) is CODE");

done_testing();
