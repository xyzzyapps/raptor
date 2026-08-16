# Raptor language grammar — gcre (Grammar Compatible Regular Expressions).
# Subset of Raku that is PEG-compatible. No action blocks.

grammar Raptor {
    rule TOP { <statement>* }

    token comment { '#' <-[\n]>* }

    # <HOST_stmt> calls the Go Pratt parser for one statement.
    # PEG alternatives below run only when Pratt cannot consume here.
    rule statement {
        | <comment>
        | <HOST_stmt>
        | <grammar_decl>
        | <package_decl>
        | <use_stmt>
        | <sub_decl>
        | <var_decl>
        | <if_stmt>
        | <unless_stmt>
        | <while_stmt>
        | <until_stmt>
        | <for_stmt>
        | <loop_stmt>
        | <given_stmt>
        | <subset_decl>
        | <struct_decl>
        | <enum_decl>
        | <return_stmt>
        | <last_stmt>
        | <next_stmt>
        | <goto_stmt>
        | <take_stmt>
        | <assert_stmt>
        | <advice_stmt>
        | <label_stmt>
        | <block>
        | <expr_stmt>
    }

    rule expr_stmt { <expression> <modifier>? ';'? }
    rule return_stmt { 'return' <expression>? <modifier>? ';'? }
    rule last_stmt { 'last' ';'? }
    rule next_stmt { 'next' ';'? }
    rule take_stmt { 'take' <expression> ';'? }
    rule goto_stmt { 'goto' <goto_target> ';'? }
    token goto_target { '&'? <name> }
    rule assert_stmt { 'assert' <expression> [ ',' <expression> ]? ';'? }

    rule modifier {
        | 'if' <expression>
        | 'unless' <expression>
        | 'while' <expression>
        | 'until' <expression>
        | 'for' <expression>
        | 'given' <expression>
    }

    rule label_stmt { <name> ':' <statement>? }

    rule var_decl {
        <scope> <typename>? <var> <where_clause>? [ '=' <expression> ]? <modifier>? ';'?
    }
    token scope { 'my' | 'our' | 'state' }
    token typename { 'Int' | 'Str' | 'Num' | 'Bool' | 'Array' | 'Hash' | 'Any' | 'Callable' }
    rule where_clause { 'where' [ <block> | <expression> ] }

    rule package_decl {
        [ 'unit' ]? [ 'package' | 'module' ] <colon_name> [ <block> | ';' ]
    }
    token colon_name { <name> [ '::' <name> ]* }

    rule use_stmt { 'use' <colon_name> [ ':from<' <-[>]>+ '>' ]? ';'? }

    rule sub_decl {
        [ 'multi' ]? 'sub' <sub_name> <sig>? [ 'returns' <typename> ]? <native_trait>* [ '{' '*' '}' | '{*}' | <block> | ';' ]
    }
    token sub_name {
        | 'infix:<' <-[>]>+ '>'
        | 'prefix:<' <-[>]>+ '>'
        | 'postfix:<' <-[>]>+ '>'
        | <name>
        | <uni_name>
    }
    token uni_name { '∑' | '∏' | '√' | '±' | 'π' | 'τ' | '÷' | '×' }
    rule native_trait { 'is' 'native' '(' <string> ')' [ 'is' 'symbol' '(' <string> ')' ]? }
    rule sig { '(' <param_list>? ')' }
    rule param_list { <param> [ ',' <param> ]* }
    rule param {
        | '[' <param_list> ']'
        | ':?'? <typename>? '*'? <var> <where_clause>?
        | ':?'? '*'? <var> <where_clause>?
    }

    rule if_stmt {
        'if' <expression> <block>
        [ 'elsif' <expression> <block> ]*
        [ 'else' <block> ]?
    }
    rule unless_stmt { 'unless' <expression> <block> }
    rule while_stmt { 'while' <expression> <block> }
    rule until_stmt { 'until' <expression> <block> }
    rule for_stmt { 'for' <expression> [ '->' <var> ]? <block> }
    rule loop_stmt {
        | 'loop' '(' <loop_part>? ';' <loop_part>? ';' <loop_part>? ')' <block>
        | 'loop' <block>
    }
    rule loop_part { <scope> <var> '=' <expression> | <expression> }
    rule given_stmt { 'given' <expression> <given_block> }
    rule given_block { '{' [ <when_clause> | <default_clause> | <statement> ]* '}' }
    rule when_clause { 'when' <expression> <block> }
    rule default_clause { 'default' <block> }

    rule subset_decl { 'subset' <name> [ 'of' <typename> ]? <where_clause> ';'? }
    rule struct_decl {
        [ 'struct' | 'union' ] <name> '{' <struct_field>* '}'
    }
    rule struct_field { <name> <var> ';'? }
    rule enum_decl { 'enum' <name> [ '<' <name>+ '>' | '(' <hash_pair>+ ')' ] ';'? }

    rule grammar_decl { 'grammar' <name> '{' <g_rule>* '}' }
    rule g_rule { [ 'token' | 'rule' | 'regex' ] <name> '{' <-[\}]>* '}' }

    rule advice_stmt { [ 'before' | 'after' | 'around' ] <name> <sig>? <block> }

    rule block { '{' <statement>* '}' }

    rule expression { <assign_expr> | <HOST_expr> }

    rule assign_expr { <postfix> <assign_op> <assign_expr> | <ternary> }
    token assign_op { '//=' | '+=' | '-=' | '~=' | '*=' | '/=' }

    rule ternary {
        <or_expr> [ '??' <expression> '!!' <expression> | '?' <expression> ':' <expression> ]?
    }

    rule or_expr { <and_expr> [ <or_op> <and_expr> ]* }
    token or_op { '||' | 'or' | 'orelse' }

    rule and_expr { <cmp_expr> [ <and_op> <cmp_expr> ]* }
    token and_op { '&&' | 'and' }

    rule cmp_expr { <add_expr> [ <cmp_op> <add_expr> ]* }
    token cmp_op {
        | '<=>' | 'cmp' | '~~' | '!~'
        | '==' | '!=' | '<=' | '>=' | 'eq' | 'ne' | 'lt' | 'gt' | 'le' | 'ge'
        | '=~' | 'min' | 'max' | '∈' | '∉'
        | '='
        | '<' | '>'
    }

    rule add_expr { <mul_expr> [ <add_op> <mul_expr> ]* }
    token add_op { '+' | '-' | '~' | '±' }

    rule mul_expr { <pow_expr> [ <mul_op> <pow_expr> ]* }
    token mul_op {
        | '%%' | '/' '/' | '+&' | '+|' | '+^' | '+<' | '+>'
        | 'div' | 'mod' | 'xx' | '×' | '÷'
        | '*' | '/' | '%' | 'x'
    }

    token starstar { '*' '*' }
    rule pow_expr { <range_expr> [ <starstar> <pow_expr> ]? }

    rule range_expr { <prefix_expr> [ '..' <prefix_expr> ]? }

    rule prefix_expr { <prefix_op>* <postfix> }
    token prefix_op { '√' | 'not' | 'so' | '\\' | '+' | '-' | '!' | '?' | '~' }

    rule postfix { <primary> <postfix_tail>* }
    rule postfix_tail {
        | '.' <name> [ '(' <arglist>? ')' ]?
        | '->' '[' <expression> ']'
        | '->' '{' <expression> '}'
        | '->' '(' <arglist>? ')'
        | '->' <name> [ '(' <arglist>? ')' ]?
        | '[' <expression> ']'
        | '{' <expression> '}'
        | '<' <name> '>'
        | '(' <arglist>? ')'
    }

    rule arglist { <expression> [ ',' <expression> ]* ','? }

    rule primary {
        | <gather_expr>
        | <start_expr>
        | <listop>
        | <anon_sub>
        | <array_lit>
        | <hash_or_block>
        | <paren>
        | <number>
        | <string>
        | <backtick>
        | <var>
        | <bare_name>
        | <uni_name>
        | '...'
        | 'True' | 'False' | 'Nil' | 'true' | 'false'
    }

    rule gather_expr { 'gather' <block> }
    rule start_expr { 'start' <block> }
    rule anon_sub { 'sub' <sig>? <block> }
    rule array_lit { '[' <arglist>? ']' }
    rule hash_or_block {
        | '{' <hash_pair> [ ',' <hash_pair> ]* ','? '}'
        | <block>
    }
    rule hash_pair {
        | ':' <name> [ '=>' <expression> ]?
        | <expression> [ '=>' | ':' ] <expression>
    }
    rule paren { '(' <expression> ')' }

    rule listop { <listop_name> <expression> [ ',' <expression> ]* }
    token listop_name {
        | 'say' | 'print' | 'die' | 'warn' | 'note' | 'push' | 'pop'
        | 'shift' | 'unshift' | 'elems' | 'join' | 'split' | 'abs'
        | 'int' | 'str' | 'chr' | 'ord' | 'sqrt' | 'sin' | 'cos'
        | 'ok' | 'is' | 'isnt' | 'like' | 'pass' | 'fail' | 'plan'
        | 'diag' | 'done-testing' | 'done_testing' | 'use_ok'
        | 'pre' | 'post' | 'invariant'
    }

    token var {
        | '$*' <name> [ '-' <name> ]*
        | '@*' <name>
        | '%*' <name>
        | '$?'
        | '$!'
        | '$$'
        | '$0'
        | '$_'
        | '$/'
        | <sigil> <colon_name>
        | <sigil>
    }
    token sigil { '$' | '@' | '%' | '&' }

    token name { <[a..zA..Z_]> [\w | ':']* }
    token bare_name { <name> }

    token number {
        | '0x' <[0..9a..fA..F]>+
        | '0b' <[01]>+
        | '0o' <[0..7]>+
        | \d+ '.' \d+ [ <[eE]> <[+\-]>? \d+ ]?
        | \d+ [ <[eE]> <[+\-]>? \d+ ]?
    }

    token string {
        | "'" <-[']>* "'"
        | '"' <dchar>* '"'
        | 'q[' <-[\]]>* ']'
        | 'qq[' <-[\]]>* ']'
    }
    token dchar { '\\' . | <-["\\]>+ }

    token backtick {
        | '`' <-[`]>* '`'
        | 'qx{' <-[\}]>* '}'
    }
}
