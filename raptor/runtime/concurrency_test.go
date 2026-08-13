package raptor

import (
	"sync"
	"testing"
	"time"
)

func TestPromiseStartAndAwait(t *testing.T) {
	in := NewInterp()
	code := `
my $p1 = start {
    sleep(0.02);
    return 42;
};

my $p2 = Promise.start(sub {
    sleep(0.01);
    return 100;
});

my $res1 = await($p1);
my $res2 = $p2.result();

[$res1, $res2];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 2 {
		t.Fatalf("expected array of 2 elements, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 42 {
		t.Errorf("p1 result mismatch: %d", val.ArrayVal[0].IntVal)
	}
	if val.ArrayVal[1].IntVal != 100 {
		t.Errorf("p2 result mismatch: %d", val.ArrayVal[1].IntVal)
	}
}

func TestChannelProducerConsumer(t *testing.T) {
	in := NewInterp()
	code := `
my $chan = Channel.new(10);
$chan.send(10);
$chan.send(20);
$chan.send(30);

my $v1 = $chan.receive();
my $v2 = $chan.receive();
my $v3 = $chan.receive();

$chan.close();

[$v1, $v2, $v3];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 3 {
		t.Fatalf("expected array of 3, got %v", val)
	}
	if val.ArrayVal[0].IntVal != 10 || val.ArrayVal[1].IntVal != 20 || val.ArrayVal[2].IntVal != 30 {
		t.Errorf("channel values mismatch: %v", val)
	}
}

func TestAtomicsParallel(t *testing.T) {
	in := NewInterp()
	code := `
my $counter = 0;
my $cas_success = atomic_cas($counter, 0, 50);
my $old_val = atomic_load($counter);
my $new_val = atomic_add($counter, 25);
my $sub_val = atomic_sub($counter, 15);

[$cas_success, $old_val, $new_val, $sub_val];
`
	val, err := in.Eval(code)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if val.Type != ValArray || len(val.ArrayVal) != 4 {
		t.Fatalf("expected array of 4, got %v", val)
	}
	if !val.ArrayVal[0].IsTrue() {
		t.Errorf("cas_success was false")
	}
	if val.ArrayVal[1].IntVal != 50 {
		t.Errorf("expected 50, got %d", val.ArrayVal[1].IntVal)
	}
	if val.ArrayVal[2].IntVal != 75 {
		t.Errorf("expected 75, got %d", val.ArrayVal[2].IntVal)
	}
	if val.ArrayVal[3].IntVal != 60 {
		t.Errorf("expected 60, got %d", val.ArrayVal[3].IntVal)
	}
}

func TestConcurrentGoroutineAtomics(t *testing.T) {
	in := NewInterp()
	// Test high concurrency atomic additions
	var wg sync.WaitGroup
	countVal := IntValue(0)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = in.Builtins["atomic_add"](in, []*Value{countVal, IntValue(1)})
				time.Sleep(time.Microsecond * 10)
			}
		}()
	}
	wg.Wait()

	if countVal.IntVal != 1000 {
		t.Errorf("expected count 1000, got %d", countVal.IntVal)
	}
}
