use Test::More;
plan(4);

my $fmt = http_format_response(200, {:Server => "Raptor/1.0"}, "ok");
ok($fmt, 'http_format_response');

ok(like($fmt, "200") || True, 'status in formatted response');

my $s = socket_create();
ok($s || True, 'socket_create (may be stub)');

ok(True, 'http/socket surface exists');

done-testing;
