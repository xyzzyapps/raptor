plan(3);

ok(True, 'docs/ is PodLit-readable');

my $idx = slurp("docs/raptor.pod");
ok($idx, 'raptor.pod readable');

my $md = pod_weave($idx);
ok($md, 'pod_weave raptor');

done_testing();
