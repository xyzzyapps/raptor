# Example NQP / QRegex declarative language grammar for gcre.
# Demonstrates multi-grammar inheritance, protoregexes (:sym<...>), LTM (| vs ||),
# declarative `is optable` Pratt precedence, separated lists (%), goal matching (~),
# lookarounds, character class set algebra, and action stub markers ({*}).

grammar ExprCore {
    token EXPR is optable { <term> }

    proto token term { * }
    token term:sym<int>   { \d+ {*} }
    token term:sym<hex>   { '0x' <xdigit>+ {*} }
    token term:sym<group> { '(' ~ ')' [ <.ws> <EXPR> <.ws> ] {*} }
    token term:sym<call>  { <id=ident_name> '(' ~ ')' [ <.ws> <arg=EXPR> ** 0..* % [ <.ws> ',' <.ws> ] <.ws> ] {*} }
    token term:sym<var>   { <id=ident_name> {*} }

    token keyword { [ 'let' | 'if' | 'return' | 'and' | 'or' ] >> }
    token ident_name { <!keyword> <[a..zA..Z_] + :Lu> <[a..zA..Z0..9_]>* }

    proto token prefix { * }
    token prefix:sym<-> is tighter<*> { <sym> }

    proto token infix { * }
    token infix:sym<+>   { <sym> }
    token infix:sym<->   is equiv<+> { <sym> }
    token infix:sym<*>   is tighter<+> { <sym> }
    token infix:sym</>   is equiv<*> { <sym> }
    token infix:sym<**>  is tighter<*> is assoc<right> { <sym> }
    token infix:sym<==>  is looser<+> is assoc<non> { <sym> }
    token infix:sym<and> is looser<==> { <sym> >> }
    token infix:sym<or>  is looser<and> { <sym> >> }
}

grammar MiniNQP is ExprCore {
    rule TOP { ^ <stmt>+ $ }

    proto rule stmt { * }
    rule stmt:sym<let>    { <sym> <lhs=ident_name> '=' <rhs=EXPR> ';' {*} }
    rule stmt:sym<return> { <sym> <val=EXPR> ';' {*} }
    rule stmt:sym<if>     { <sym> '(' ~ ')' <cond=EXPR> '{' ~ '}' <stmt>* {*} }
    rule stmt:sym<expr>   { <EXPR> ';' {*} }
}
