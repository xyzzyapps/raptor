# ==============================================================================
# TAP Test Suite 06: PodLit Literate Programming (Weave, Tangle, Mangle)
# ==============================================================================

plan(9);

my $doc = '=pod

=head1 Literate Math

A literate document defining arithmetic functions.

=chunk <add-func> :file "lib/Math/Add.rp"
sub add($x, $y) {
    return $x + $y;
}
=end chunk

=chunk <main> :file "bin/calc.rp"
<<add-func>>

my $res = add(10, 32);
=end chunk

=cut
';

# 1. Weave to Markdown
my $md = pod_weave($doc);
ok(length($md) > 0, "pod_weave produced non-empty markdown");
like($md, "# Literate Math", "woven markdown contains heading");
like($md, "«add-func»", "woven markdown contains chunk reference");

# 2. Tangle to source code
my %files = pod_tangle($doc);
ok(%files{"lib/Math/Add.rp"} // "" ne "", "tangled lib/Math/Add.rp");
ok(%files{"bin/calc.rp"} // "" ne "", "tangled bin/calc.rp");

my $mainCode = %files{"bin/calc.rp"} // "";
like($mainCode, "sub add", "tangled main code contains expanded add-func");

# 3. Stitch modified source back into POD
my %stitchMap = {};
%stitchMap{"lib/Math/Add.rp"} = 'sub add($x, $y) { return ($x + $y) * 10; }';

my $stitched = pod_stitch($doc, %stitchMap);
ok(length($stitched) > 0, "pod_stitch produced updated pod");
like($stitched, "=head1 Literate Math", "stitched pod preserved headers");
like($stitched, "sub add.*10", "stitched pod contains updated implementation");

done_testing();
