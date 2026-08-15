plan(3);

use "t/modules/AutoService.rp";

is(AutoService::greet("World"), "Hello, World!", "normal sub from used module succeeds");
is(AutoService::missing_op(10, 20), "AutoService::AUTOLOAD -> AutoService::missing_op (10, 20)", "missing sub in used module triggers its AUTOLOAD");
is(AutoService::another_unknown(5, 7), "AutoService::AUTOLOAD -> AutoService::another_unknown (5, 7)", "second missing sub triggers AUTOLOAD");

done_testing();
