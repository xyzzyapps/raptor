package raptor

import (
	"bytes"
	"strings"
	"testing"

	moargo "moarvm-go/engine"
)

func TestAuditCallDisambiguationAndParens(t *testing.T) {
	in := NewInterp()
	var buf bytes.Buffer
	in.SetStdout(&buf)
	src := `
sub fib($n) {
    if $n <= 1 { return $n; }
    return (fib($n - 1) + fib($n - 2));
}
sub pair($a, $b) { return $a + $b; }

my $val = 100;
my $sref = \$val;
is(ref($sref), "SCALAR", "adjacent nested call");
is($$sref, 100, "bare dollar deref inside call");
is(fib(6), 8, "parenthesized binary sum of fib calls");
is((pair 5, 6), 11, "prefix call in parens");
is (1 + 1), 2, "TAP word with space before paren";
say (1 + 2) * 3;
say (pair 7, 8);
PRE(1 != 0, "call-form PRE works");
ASSERT(2 + 2 == 4, "call-form ASSERT works");
`
	if _, err := in.Eval(src); err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, buf.String())
	}
	out := buf.String()
	if strings.Contains(out, "not ok") {
		t.Fatalf("TAP assertion failed:\n%s", out)
	}
	if !strings.Contains(out, "\n9\n15\n") {
		t.Fatalf("expected say outputs 9 and 15, got:\n%s", out)
	}
}

func TestAuditOperatorsAssociativityAndLiterals(t *testing.T) {
	in := NewInterp()
	src := `
my $pow = 2 ** 3 ** 2;
my $sm = (2 + 3 ~~ Int);
my $oct = 0o755;
my $sci = 1.5e3;
my $le1 = ("abc" le "abd") && ("abc" le "abc") && !("abd" le "abc");
my $ge1 = ("abd" ge "abc") && ("abc" ge "abc") && !("abc" ge "abd");
my $cmp1 = ("a" cmp "b") == -1 && ("b" cmp "b") == 0 && ("c" cmp "b") == 1;
my $ncmp = (10 <=> 20) == -1 && (20 <=> 20) == 0 && (30 <=> 20) == 1;
my $xor1 = (True xor False) && (False xor True) && !(True xor True) && !(False xor False);
my $def = Nil orelse 42;
my $m = 6;
$m *= 7;
my $d = 20;
$d /= 4;
my $esc = "$m costs \$99 and \{raw} pid=$$ exit=$?";
[$pow, $sm, $oct, $sci, $le1, $ge1, $cmp1, $ncmp, $xor1, $def, $m, $d, $esc];
`
	res, err := in.Eval(src)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if res.Type != ValArray || len(res.ArrayVal) != 13 {
		t.Fatalf("unexpected result: %v", res)
	}
	if res.ArrayVal[0].FloatVal != 512 {
		t.Errorf("2 ** 3 ** 2 = %v, want 512", res.ArrayVal[0])
	}
	if !res.ArrayVal[1].BoolVal {
		t.Errorf("2 + 3 ~~ Int = false, want true")
	}
	if res.ArrayVal[2].IntVal != 493 {
		t.Errorf("0o755 = %v, want 493", res.ArrayVal[2])
	}
	if res.ArrayVal[3].FloatVal != 1500 {
		t.Errorf("1.5e3 = %v, want 1500", res.ArrayVal[3])
	}
	for idx, name := range []string{"le", "ge", "cmp", "<=>", "xor"} {
		if !res.ArrayVal[4+idx].BoolVal {
			t.Errorf("operator %s check failed", name)
		}
	}
	if res.ArrayVal[9].IntVal != 42 {
		t.Errorf("orelse = %v, want 42", res.ArrayVal[9])
	}
	if res.ArrayVal[10].IntVal != 42 {
		t.Errorf("*= = %v, want 42", res.ArrayVal[10])
	}
	if res.ArrayVal[11].FloatVal != 5 {
		t.Errorf("/= = %v, want 5", res.ArrayVal[11])
	}
	esc := res.ArrayVal[12].StrVal
	if !strings.HasPrefix(esc, "42 costs $99 and {raw} pid=") || !strings.HasSuffix(esc, " exit=0") {
		t.Errorf("unexpected interpolated string: %q", esc)
	}
}

func TestAuditPackedIntRangeArrayEverywhere(t *testing.T) {
	in := NewInterp()
	src := `
my @r = 1..5;
my $truth = ?@r;
my $json = to_json(@r);
my $sum = ∑(@r);
my $prod = ∏(@r);
my $len = +@r;
my $str = ~@r;
my $joined = join("-", @r);
my $has3 = (3 ∈ @r) && (9 ∉ @r);
my @inter = (1..4) ∩ (3..6);
my @uni = (1..2) ∪ (2..3);
my @rep = (1..2) xx 2;
my @rev = reverse(@r);
my $hd = head(@r);
my @tl = tail(@r);
my @mapped = map(sub ($x) { return $x * 2; }, @r);
my @grepped = grep(sub ($x) { return $x > 3; }, @r);
my $red = reduce(sub ($a, $b) { return $a + $b; }, @r);
my $juncOk = ?(any(@r) == 4) && ?(all(@r) > 0);

my $modSum = 0;
$modSum += $_ for 1..4;

my $aref = \@r;
my $at1 = $aref->[1];

sub destruct([$first, *@rest]) {
    return $first * 100 + elems(@rest);
}
my $dval = destruct(10..13);

my @mut = 1..4;
my $p = pop(@mut);
my $sh = shift(@mut);
unshift(@mut, 99);

[$truth, $json, $sum, $prod, $len, $str, $joined, $has3, elems(@inter), elems(@uni), elems(@rep), @rev[0], $hd, elems(@tl), @mapped[4], elems(@grepped), $red, $juncOk, $modSum, $at1, $dval, $p, $sh, @mut[0]];
`
	res, err := in.Eval(src)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	a := res.ArrayVal
	if !a[0].BoolVal {
		t.Errorf("packed range truthiness = false")
	}
	if a[1].StrVal != "[1,2,3,4,5]" {
		t.Errorf("to_json(1..5) = %q", a[1].StrVal)
	}
	if a[2].IntVal != 15 || a[3].IntVal != 120 {
		t.Errorf("sum/prod = %v / %v", a[2], a[3])
	}
	if a[4].IntVal != 5 || a[5].StrVal != "1 2 3 4 5" || a[6].StrVal != "1-2-3-4-5" {
		t.Errorf("coercions/join = %v / %q / %q", a[4], a[5].StrVal, a[6].StrVal)
	}
	if !a[7].BoolVal || a[8].IntVal != 2 || a[9].IntVal != 3 || a[10].IntVal != 4 {
		t.Errorf("set/xx ops failed: %v %v %v %v", a[7], a[8], a[9], a[10])
	}
	if a[11].IntVal != 5 || a[12].IntVal != 1 || a[13].IntVal != 4 {
		t.Errorf("rev/head/tail failed: %v %v %v", a[11], a[12], a[13])
	}
	if a[14].IntVal != 10 || a[15].IntVal != 2 || a[16].IntVal != 15 || !a[17].BoolVal {
		t.Errorf("HOFs/junctions failed: %v %v %v %v", a[14], a[15], a[16], a[17])
	}
	if a[18].IntVal != 10 || a[19].IntVal != 2 || a[20].IntVal != 1003 {
		t.Errorf("modFor/deref/destruct failed: %v %v %v", a[18], a[19], a[20])
	}
	if a[21].IntVal != 4 || a[22].IntVal != 1 || a[23].IntVal != 99 {
		t.Errorf("pop/shift/unshift failed: %v %v %v", a[21], a[22], a[23])
	}
}

func TestAuditWhereConstraintNotBypassedByInPlaceMutation(t *testing.T) {
	in := NewInterp()
	_, err := in.Eval(`
my Int $pct where { $_ >= 0 && $_ <= 100 } = 50;
$pct += 100;
`)
	if err == nil {
		t.Fatalf("expected where constraint violation on $pct += 100, got nil")
	}

	in2 := NewInterp()
	_, err = in2.Eval(`
my Str $short where { $_.chars() <= 3 } = "ab";
$short ~= "cde";
`)
	if err == nil {
		t.Fatalf("expected where constraint violation on $short ~= 'cde', got nil")
	}
}

func TestAuditUnaliasedIntBoxesAndHashKeys(t *testing.T) {
	in := NewInterp()
	res, err := in.Eval(`
my $a = 1000;
my $b = $a;
$a += 1;
my $c = 0;
$c = $a;
$a += 10;

my %h = { 1 => "one", "cnt" => 10 };
%h{2} = "two";
%h{"cnt"} += 5;
my @arr = [10, 20];
@arr[0] += 7;
my $p5style = $arr[1];
[$a, $b, $c, %h{1}, %h{2}, %h{"cnt"}, @arr[0], $p5style];
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if res.ArrayVal[0].IntVal != 1011 || res.ArrayVal[1].IntVal != 1000 || res.ArrayVal[2].IntVal != 1001 {
		t.Fatalf("int box aliasing bug: got %v", res)
	}
	if res.ArrayVal[3].StrVal != "one" || res.ArrayVal[4].StrVal != "two" {
		t.Fatalf("numeric hash key bug: got %v", res)
	}
	if res.ArrayVal[5].IntVal != 15 || res.ArrayVal[6].IntVal != 17 || res.ArrayVal[7].IntVal != 20 {
		t.Fatalf("compound index/hash assign or Perl5 sigil indexing bug: got %v", res)
	}
}

func TestAuditMoarXorAndOperators(t *testing.T) {
	if moargo.FindMoarDLL() == "" {
		t.Skip("moar.dll not found")
	}
	in := NewInterp()
	var buf bytes.Buffer
	in.SetStdout(&buf)
	src := `
my $x = 6;
$x *= 7;
say $x;
say (1 xor 0);
say (1 xor 1);
say ("abc" le "abd");
`
	if _, err := in.EvalOnBackend(src, BackendMoar); err != nil {
		t.Fatalf("moar error: %v", err)
	}
	got := strings.Fields(buf.String())
	want := []string{"42", "1", "0", "1"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("moar got %v, want %v", got, want)
	}
}

func TestAuditDynamicNQPGrammarInRaptor(t *testing.T) {
	in := NewInterp()
	var buf bytes.Buffer
	in.SetStdout(&buf)
	src := `
grammar PointGrammar {
    rule TOP { <point> }
    rule point { '(' ~ ')' [ <number>+ % ',' ] }
    token number { \d+ }
}

my $m = PointGrammar.parse("(10, 20)");
say $m<point><number>[0]<Str>;
say $m<point><number>[1]<Str>;
say $/<point><Str>;

my $bad = PointGrammar.parse("(10, 20) trailing");
say defined($bad) ?? "bad" !! "rejected";

my $sub = PointGrammar.subparse("(30, 40) trailing");
say $sub<point><number>[0]<Str>;

grammar BaseLang {
    rule TOP { ^ <EXPR> $ }
    token EXPR is optable { <term> }
    proto token term { * }
    token term:sym<num> { \d+ }
    proto token infix { * }
    token infix:sym<+> { <sym> }
    token infix:sym<*> is tighter<+> { <sym> }
}

grammar ExtLang is BaseLang {
    token term:sym<str> { '"' <-["]>* '"' }
    token infix:sym<~> is equiv<+> { <sym> }
}

my $em = ExtLang.parse('"hi" ~ 2 * 3');
say $em<EXPR><sym><Str>;
say $em<EXPR><right><Str>;
`
	if _, err := in.Eval(src); err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	got := strings.TrimSpace(strings.ReplaceAll(buf.String(), "\r\n", "\n"))
	want := strings.Join([]string{
		"10",
		"20",
		"(10, 20)",
		"rejected",
		"30",
		"~",
		"2 * 3",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAuditScalarBraceHashAccess(t *testing.T) {
	in := NewInterp()
	var buf bytes.Buffer
	in.SetStdout(&buf)
	src := `
sub make_cfg() { return { "host" => "localhost", "nested" => { "port" => 8080 } }; }
my $cfg = make_cfg();
say $cfg{"host"};
say $cfg{"nested"}{"port"};
say make_cfg(){"host"};
if $cfg {
    say "block_ok";
}
`
	if _, err := in.Eval(src); err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	got := strings.TrimSpace(strings.ReplaceAll(buf.String(), "\r\n", "\n"))
	want := "localhost\n8080\nlocalhost\nblock_ok"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
