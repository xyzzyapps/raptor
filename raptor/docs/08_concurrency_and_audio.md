# Raptor Concurrency, Atomics & PortAudio Audio Engine

Raptor provides high-performance asynchronous concurrency, thread synchronization, hardware atomics, and real-time audio synthesis.

---

## 1. Asynchronous Tasks & Promises

```perl
# Asynchronous promise
my $p = start {
    # Long running computation
    my $sum = 0;
    for 1..1000 -> $i { $sum = $sum + $i; }
    return $sum;
};

# Await promise result
my $result = await($p);
say "Result: " ~ $result;
```

---

## 2. Channels & Queues

```perl
my $chan = Channel.new(10);

# Producer
start {
    for 1..5 -> $i {
        $chan.send($i * 10);
    }
};

# Consumer
for 1..5 {
    my $val = $chan.receive();
    say "Received: " ~ $val;
}
```

---

## 3. Parallel Map

```perl
my @inputs = [1, 2, 3, 4, 5, 6, 7, 8];
my @results = parallel_map(@inputs, sub ($x) {
    return $x * $x;
}, 4); # 4 worker threads

say "Parallel Squares: " ~ @results;
```

---

## 4. Hardware Atomics & Mutexes

```perl
my $val = 100;
atomic_add($val, 50);
say "Atomic Val: " ~ $val; # 150

my $mtx = mutex_create();
mutex_lock($mtx);
# Critical section
mutex_unlock($mtx);
```

---

## 5. PortAudio Real-Time Synthesizer

```perl
# Initialize PortAudio driver
pa_init();

my $dev_count = pa_device_count();
say "Audio devices found: " ~ $dev_count;

# Synthesize a 440 Hz (Concert A) sine wave buffer
my $samples = pa_sine_wave(440.0, 0.5, 44100, 0.8);

pa_terminate();
```
