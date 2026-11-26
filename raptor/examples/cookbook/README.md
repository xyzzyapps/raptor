# Perl Cookbook — Complete 20-Chapter Port to Raptor

This directory contains idiomatic, self-verifying **Raptor** ports covering all **20 chapters** of O'Reilly's *Perl Cookbook* (Tom Christiansen & Nathan Torkington). Every script executes standalone and verifies its results with `ASSERT` contracts.

## Running the Cookbook Suite

```powershell
# Run an individual chapter
./bin/raptor.exe --go examples/cookbook/01_strings.rp

# Run all 20 chapters
Get-ChildItem examples/cookbook/*.rp | Sort-Object Name | ForEach-Object {
    ./bin/raptor.exe --go $_.FullName
}
```

## Chapter Index

| Chapter | Script | Highlights & Recipes Covered |
| :---: | :--- | :--- |
| **01** | [`01_strings.rp`](01_strings.rp) | `substr`, `pack`/`unpack`, `ord`/`chr`/`codes`, `flip`/`reverse`, tab expand/compress, `{expr}` interpolation, `CSVGrammar`, Soundex |
| **02** | [`02_numbers.rp`](02_numbers.rp) | Numeric validation, float tolerance, radix literals (`0x`/`0b`/`0o`), `hex`/`oct`, Roman numerals, `srand`/`rand`, trig/logs, matrix multiply, commify |
| **03** | [`03_dates_and_times.rp`](03_dates_and_times.rp) | Epoch conversions (`gmtime`/`localtime`), Julian/Rata Die day math, date arithmetic, weekday names, ISO-8601 PEG grammar, `now()` high-res timers |
| **04** | [`04_arrays.rp`](04_arrays.rp) | `qw(...)`, `<word lists>`, `splice`, set union/intersection/difference, `unique`, `grep`/`map`/`reduce`, Fisher-Yates shuffle, permutations |
| **05** | [`05_hashes.rp`](05_hashes.rp) | `exists`/`delete`, deterministic key/value sorting, hash inversion, multi-value HoA accumulators, frequency counting (`%freq{$w}++`), hash merge & diff |
| **06** | [`06_pattern_matching.rp`](06_pattern_matching.rp) | `=~` / `!~`, `$1`/`$2` & named captures `$<name>`, `match_all`, `subst`, lookarounds, recursive balanced-delimiter Raku PEG grammar |
| **07** | [`07_file_access.rp`](07_file_access.rp) | `open`, `<$fh>` diamond `readline`, `close`, `slurp`/`spurt`, `-e`/`-f`/`-d`/`-s`/`-r`/`-w` file test operators, atomic temp-file + `rename`, `copy` |
| **08** | [`08_file_contents.rp`](08_file_contents.rp) | Backslash continuation lines, line/word/paragraph counts, reverse lines/paragraphs, reservoir random-line sampling, binary fixed-length records, INI parser |
| **09** | [`09_directories.rp`](09_directories.rp) | `mkdir`, `dir`, recursive directory tree walker, glob-style filtering, safe batch rename, subtree copy and cleanup |
| **10** | [`10_subroutines.rp`](10_subroutines.rp) | Default parameters, slurpy `*@rest`/`*%opts`, `@_`, list destructuring returns, `state` persistent variables, closures, `try`/`catch` (`$!`), `multi sub` |
| **11** | [`11_references_and_records.rp`](11_references_and_records.rp) | Scalar/array/hash/code references (`\$x`, `$$r`, `@$ar`, `%$hr`, `&$cr`), nested HoH/AoH records, closures as objects, circular graphs, JSON persistence |
| **12** | [`12_packages_and_modules.rp`](12_packages_and_modules.rp) | `package` namespaces, `our` globals, qualified calls, symbol table introspection (`package_symbols`, `package_get`, `package_set`), `PRE` contracts |
| **13** | [`13_structs_and_dispatch.rp`](13_structs_and_dispatch.rp) | Zero-copy `CStruct` records, factory constructors, UFCS method calls, `multi sub` polymorphic dispatch, custom `infix:` operator overloading, `before`/`after` AOP advice |
| **14** | [`14_database_access.rp`](14_database_access.rp) | `sqlite_open` (`:memory:` & file DB), DDL schema creation, transactional CRUD, aggregate queries, hash persistence table |
| **15** | [`15_user_interfaces_tui.rp`](15_user_interfaces_tui.rp) | Command-line flag/option parser, ANSI 256-color helpers, ASCII box table renderer, progress bar formatter, text sparklines/bar charts |
| **16** | [`16_process_management_ipc.rp`](16_process_management_ipc.rp) | `setenv`/`getenv`/`unsetenv`, `qx` subprocess output capture, `start`/`await` concurrent Promises, buffered `Channel` worker pool, multi-core `pmap` |
| **17** | [`17_sockets.rp`](17_sockets.rp) | Loopback `tcp_listen`/`tcp_accept`/`tcp_connect` request/response server & client, `udp_bind`/`udp_sendto`/`udp_recvfrom` datagram sockets |
| **18** | [`18_internet_services.rp`](18_internet_services.rp) | `ServiceURI` PEG grammar, RFC 5322 / SMTP MIME Base64 message builder, HMAC-SHA256 signed service tokens, local HTTP WHOIS service |
| **19** | [`19_cgi_and_web.rp`](19_cgi_and_web.rp) | `application/x-www-form-urlencoded` parser/encoder, `html_escape` XSS protection, `Cookie` jar & `http_parse_request`/`http_format_response`, live form POST |
| **20** | [`20_web_automation.rp`](20_web_automation.rp) | HTML `href` extractor, HTML-to-plain-text converter, breadth-first local site crawler & broken-link detector, Common Log Format (`CLFLog`) analyzer |
