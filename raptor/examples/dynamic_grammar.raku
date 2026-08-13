# Dynamic Grammar Example in Raku Syntax
# This grammar can be loaded and executed directly by the runtime engine

grammar PointGrammar {
    rule TOP { <point> }
    rule point { '(' <number> ',' <number> ')' }
    token number { <\d+> }
}

say "PointGrammar schema loaded successfully.";
