# 25_nqp_grammar_and_edge_cases.t
# Tests embedded NQP / QRegex declarative grammars and audited runtime edge cases.

plan 20;

# 1. Multi-grammar inheritance, protoregexes (:sym<...>), and `is optable` precedence
grammar BaseCalc {
    token EXPR is optable { <term> }

    proto token term { * }
    token term:sym<int>   { \d+ }
    token term:sym<var>   { <ident> }
    token term:sym<group> { '(' ~ ')' [ <.ws> <EXPR> <.ws> ] }

    proto token prefix { * }
    token prefix:sym<-> is tighter<*> { <sym> }

    proto token infix { * }
    token infix:sym<+>  { <sym> }
    token infix:sym<*>  is tighter<+> { <sym> }
    token infix:sym<**> is tighter<*> is assoc<right> { <sym> }
}

grammar LangCalc is BaseCalc {
    rule TOP { ^ <stmt>+ % ';' ';'? $ }

    token term:sym<hex> { '0x' <xdigit>+ }

    proto rule stmt { * }
    rule stmt:sym<let>   { <sym> <lhs=ident> '=' <rhs=EXPR> }
    rule stmt:sym<print> { <sym> <arg=EXPR> }
}

my $m = LangCalc.parse("let x = 0x10 + 2 * 3 ** 2; print (x + 1);");
ok defined($m), "LangCalc inherited grammar parsed multi-statement input";
is $/{"str"}, "let x = 0x10 + 2 * 3 ** 2; print (x + 1);", "\$/ captures full matched input";

my @stmts = $m{"stmt"};
is @stmts.elems(), 2, "separated list (%) captured 2 statements";
is @stmts[0]{"rule"}, "stmt:sym<let>", "first statement dispatched to stmt:sym<let>";
is @stmts[0]{"lhs"}{"str"}, "x", "named capture alias <lhs=ident> bound";
is @stmts[0]{"rhs"}{"sym"}{"str"}, "+", "is optable parsed + as outer op over * and **";
is @stmts[1]{"rule"}, "stmt:sym<print>", "second statement dispatched to stmt:sym<print>";

# 2. LTM (`|`) vs sequential (`||`), lookarounds, and character class set algebra
grammar LexCheck {
    token kw_ltm { 'in' | 'int' | 'interface' }
    token kw_seq { 'in' || 'int' || 'interface' }
    token ident_consonants { ^ <[a..z] - [aeiou]>+ $ }
    token look_pair { ^ (\w+) <?before '='> '=' (\d+) $ }
}

my $ltm = LexCheck.subparse("interface", "kw_ltm");
is $ltm{"str"}, "interface", "LTM alternation (|) selects longest declarative match";

my $seq = LexCheck.subparse("interface", "kw_seq");
is $seq{"str"}, "in", "sequential alternation (||) selects first matching branch";

ok defined(LexCheck.parse("rhythm", "ident_consonants")), "character class difference <[a..z] - [aeiou]> matches consonants";
ok !defined(LexCheck.parse("raptor", "ident_consonants")), "character class difference rejects vowels";

my $lp = LexCheck.parse("port=8080", "look_pair");
is $lp{"pos"}[0]{"str"} ~ ":" ~ $lp{"pos"}[1]{"str"}, "port:8080", "positional captures and <?before> lookahead work";

# 3. Audited runtime & parser edge cases
my @rng = 1..5;
ok ?@rng, "packed 1..5 int array is truthy";
is ∑(@rng), 15, "sum over packed 1..5 range is 15";
is ∏(@rng), 120, "product over packed 1..5 range is 120";
is join("-", reverse(@rng)), "5-4-3-2-1", "reverse + join on packed range";

my @grid = [10, 20, 30];
@grid[1] += 7;
my %counts = { "hits" => 4 };
%counts{"hits"} *= 3;
is @grid[1] + %counts{"hits"}, 39, "compound assignment on array index and hash key";

is 2 ** 3 ** 2, 512, "exponentiation ** is right-associative";
ok ("apple" le "apple") && ("beta" ge "alpha") && (True xor False), "le, ge, and xor operators hold";

my $fallback = Nil orelse 99;
is $fallback, 99, "low-precedence orelse returns RHS for Nil";

done_testing;
