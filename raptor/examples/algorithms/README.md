# Mastering Algorithms with Perl — Complete 14-Chapter Port to Raptor

This directory contains idiomatic, self-verifying **Raptor** ports of the classic algorithms and data structures from O'Reilly's *Mastering Algorithms with Perl* (Jon Orwant, Jarkko Hietaniemi & John Macdonald), augmented with modern Raptor features (declarative PEG grammars, `state` memoization, columnar SIMD/radix sorting, and `ASSERT` verification).

## Running the Algorithms Suite

```powershell
# Run an individual chapter
./bin/raptor.exe --go examples/algorithms/08_graphs.rp

# Run all 14 chapters
Get-ChildItem examples/algorithms/*.rp | Sort-Object Name | ForEach-Object {
    ./bin/raptor.exe --go $_.FullName
}
```

## Chapter Index

| Chapter | Script | Algorithms & Data Structures Implemented |
| :---: | :--- | :--- |
| **01** | [`01_intro_complexity_bsearch.rp`](01_intro_complexity_bsearch.rp) | Iterative & recursive binary search, `lower_bound`/`upper_bound`, exponential (galloping) search, `state` memoized Fibonacci, Kadane's $O(n)$ maximum subarray |
| **02** | [`02_basic_data_structures.rp`](02_basic_data_structures.rp) | Singly linked list (with in-place reversal), doubly linked list, circular linked list (Josephus problem), LIFO stack, compacted FIFO queue, power-of-two ring-buffer deque ($O(n)$ monotonic sliding-window max) |
| **03** | [`03_advanced_data_structures.rp`](03_advanced_data_structures.rp) | Binary search tree (insert/delete/traversals), self-balancing AVL tree, binary min-heap priority queue, prefix trie, chained hash table with resizing, Disjoint-Set (Union-Find with path compression & union by rank), B-Tree |
| **04** | [`04_sorting.rp`](04_sorting.rp) | Insertion, selection, bubble, Shellsort (Knuth gaps), mergesort, quicksort, in-place heapsort, counting sort, LSD radix sort, Perl Schwartzian transform, Raptor `sort` & multi-core `psort` |
| **05** | [`05_searching.rp`](05_searching.rp) | Linear & sentinel search, interpolation search, jump search, ternary unimodal search, Quickselect $k$-th order statistic, Minimax game-tree search with Alpha-Beta pruning (Tic-Tac-Toe), A* shortest path on 2D obstacle grid |
| **06** | [`06_sets.rp`](06_sets.rp) | Hash-backed sets (union, intersection, difference, symmetric difference, subset), dense 64-bit bitvector sets, multisets (bags), power-set & $k$-subset generation, fuzzy Jaccard similarity |
| **07** | [`07_matrices.rp`](07_matrices.rp) | Dense matrix arithmetic & transpose, SIMD-accelerated dot/row products, LU decomposition with partial pivoting, Gaussian elimination solver $Ax = b$, Gauss-Jordan matrix inverse $A^{-1}$, determinant, CSR sparse matrix-vector multiply |
| **08** | [`08_graphs.rp`](08_graphs.rp) | BFS shortest unweighted path, Kahn's topological sort & cycle detection, Tarjan's Strongly Connected Components (SCC), Kruskal & Prim Minimum Spanning Tree, Dijkstra, Bellman-Ford (negative edges), Floyd-Warshall all-pairs shortest paths |
| **09** | [`09_strings_and_matching.rp`](09_strings_and_matching.rp) | Knuth-Morris-Pratt (KMP), Boyer-Moore-Horspool, Rabin-Karp rolling hash, Wagner-Fischer Levenshtein edit distance, Longest Common Subsequence (LCS), suffix array, Raku PEG grammar + action table evaluator |
| **10** | [`10_computational_geometry.rp`](10_computational_geometry.rp) | 2D cross/dot product & CCW orientation, proper/collinear segment intersection, Andrew's Monotone Chain Convex Hull, Shoelace polygon area, Ray-Casting point-in-polygon, closest pair of points |
| **11** | [`11_number_theory.rp`](11_number_theory.rp) | Euclidean GCD/LCM, Extended Euclidean Bezout identity, modular multiplicative inverse, binary `mod_pow`, Sieve of Eratosthenes, deterministic Miller-Rabin primality test (with Carmichael numbers), Chinese Remainder Theorem, Euler's totient $\varphi(n)$ |
| **12** | [`12_cryptography.rp`](12_cryptography.rp) | Caesar & Vigenère ciphers, symmetric PRNG XOR stream cipher, Diffie-Hellman key exchange, textbook RSA keygen/encrypt/decrypt, NIST SHA-256 & HMAC-SHA256 authentication |
| **13** | [`13_probability_and_statistics.rp`](13_probability_and_statistics.rp) | Mean, median, population/sample variance & standard deviation, Pearson correlation $r$, OLS simple linear regression, exact $nPr$/$nCr$ & binomial PMF, Fisher-Yates shuffle, Monte Carlo $\pi$ |
| **14** | [`14_dynamic_programming_compression.rp`](14_dynamic_programming_compression.rp) | 0/1 Knapsack, unbounded minimum coin change, CLRS Matrix Chain Multiplication, $O(n \log n)$ Longest Increasing Subsequence (LIS), Run-Length Encoding (RLE), lossless Huffman coding, lossless LZW compression |
