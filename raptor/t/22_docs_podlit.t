plan(3);

ok(True, 'docs/ is PodLit-readable');

my $idx = slurp("docs/01_raptor.pod");
ok($idx, '01_raptor.pod readable');

my $md = pod_weave($idx);
ok($md, 'pod_weave 01_raptor');

done_testing();
