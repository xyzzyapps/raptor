# Raptor Subsets, Refinement Types & Predicate Dispatching

Raptor enforces dynamic value contracts through Raku's `subset` system and **Predicate Dispatching** (ECOOP 1998).


## 1. Defining Dynamic Refinement Types (`subset`)

A `subset` defines a named invariant validated at runtime using a `where` predicate:

```perl
subset Positive where { $_ > 0 };
subset Even where { $_ % 2 == 0 };
subset PortNumber where { $_ >= 1 && $_ <= 65535 };
subset NonEmptyStr where { $_.chars > 0 };
```


## 2. Variable Contract Enforcement

Attaching a `subset` to a variable declaration creates a strict runtime contract:

```perl
my Positive $score = 100; # OK

# Attempting to assign an invalid value raises a runtime error:
# $score = -5; # Error: dynamic constraint violated for subset Positive
```


## 3. Predicate Dispatching for Multi Subs

In multi-method dispatch, candidates are disambiguated by dynamically evaluating their `where` predicates and `subset` types against incoming argument values:

```perl
subset Negative where { $_ < 0 };
subset Zero where { $_ == 0 };
subset Positive where { $_ > 0 };

# Dispatched on satisfying predicate
multi sub describe(Negative $n) { return "Negative: " ~ $n; }
multi sub describe(Zero $n)     { return "Zero"; }
multi sub describe(Positive $n) { return "Positive: " ~ $n; }

say describe(-42); # "Negative: -42"
say describe(0);   # "Zero"
say describe(100); # "Positive: 100"

# Inline where clauses in parameter signatures
multi sub grade($score where { $score >= 90 }) { return "A"; }
multi sub grade($score where { $score >= 75 && $score < 90 }) { return "B"; }
multi sub grade($score where { $score < 75 }) { return "C"; }
```


## 4. Smart Matching (`~~`)

Subsets can be matched dynamically using smart match:

```perl
if 42 ~~ Even {
    say "42 is even!";
}
```
