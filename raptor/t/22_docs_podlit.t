plan(3);

ok(True, 'docs/ is PodLit-readable');

my $idx = slurp("docs/perlraptor.pod");
ok($idx, 'perlraptor.pod readable');

my $md = pod_weave($idx);
ok($md, 'pod_weave perlraptor');

done_testing();
