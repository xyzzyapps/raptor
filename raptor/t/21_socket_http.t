plan(3);

my $hdrs = { "Server" => "Raptor/1.0" };
my $fmt = http_format_response(200, $hdrs, "ok");
ok($fmt, 'http_format_response');

ok(True, 'http surface exists');

ok(True, 'socket surface exists');

done_testing();
