# ==============================================================================
# Tcl Declarative Grammar in Raku / NQP Notation
# ==============================================================================

grammar Tcl {
    rule TOP {
        <command_line>*
    }

    rule command_line {
        | <comment>
        | <command>
    }

    token comment {
        '#' <-[\n]>*
    }

    rule command {
        <word>+
    }

    token word {
        | <braced_word>
        | <quoted_word>
        | <cmd_subst>
        | <var_subst>
        | <bare_word>
    }

    token braced_word {
        '{' <braced_content>* '}'
    }

    token braced_content {
        | <-[\{\}]>+
        | '{' <braced_content>* '}'
    }

    token quoted_word {
        '"' <quoted_content>* '"'
    }

    token quoted_content {
        | <cmd_subst>
        | <var_subst>
        | <escape_seq>
        | <-["\\\$\[]>+
    }

    token cmd_subst {
        '[' <command_line>* ']'
    }

    token var_subst {
        '$' [<ident> | '{' <ident> '}' | <ident> '(' <ident> ')']
    }

    token escape_seq {
        '\\' .
    }

    token bare_word {
        <-[\s;\[\]\{\}"\$]>+
    }

    token ident {
        <[a..zA..Z0..9_:\-]>+
    }
}
