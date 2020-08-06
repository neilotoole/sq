grammar SLQ;

// "@mysql_db1 | .user, .address | join(.user.uid == .address.uid) | .[0:3] | .uid, .username, .country"

stmtList
 : ';'* query ( ';'+ query )* ';'*
 ;

query: segment ('|' segment)* ;

segment: (element)  (',' element)* ;

element: dsTblElement | dsElement | selElement | join | group | rowRange | fn | expr;

cmpr: LT_EQ | LT | GT_EQ | GT | EQ | NEQ ;


//whereExpr
// : expr
// ;

fn: fnName '(' ( expr ( ',' expr )* | '*' )? ')';



join
 : ('join'|'JOIN'|'j')
 '(' joinConstraint ')'
 ;


joinConstraint
 : SEL cmpr SEL // .user.uid == .address.userid
 | SEL // .uid
 ;


group
  : ('group'|'GROUP'|'g')
  '(' SEL  (',' SEL)* ')'
  ;


selElement: SEL;

dsTblElement: DATASOURCE SEL; // data source table element, e.g. @my1.user

dsElement: DATASOURCE; // data source element, e.g. @my1


// []       select all rows
// [10]     select row 10
// [10:15]  select rows 10 thru 15
// [0:15]   select rows 0 thru 15
// [:15]    same as above (0 thru 15)
// [10:]    select all rows from 10 onwards
rowRange
 : '.['
 ( NN COLON NN 	// [10:15]
 | NN COLON		// [10:]
 | COLON NN		// [:15]
 | NN			// [10]
 )? ']'
 ;


fnName
 : 'sum' | 'SUM'
 | 'avg' | 'AVG'
 | 'count' | 'COUNT'
 | 'where' | 'WHERE'
 ;


expr
 : SEL
 | literal
 | unaryOperator expr
 | expr '||' expr
 | expr ( '*' | '/' | '%' ) expr
 | expr ( '+' | '-' ) expr
 | expr ( '<<' | '>>' | '&' ) expr
 | expr ( '<' | '<=' | '>' | '>=' ) expr
 | expr (  '==' | '!=' | ) expr
 | expr '&&' expr
 | fn
// | fnName '(' ( expr ( ',' expr )* | '*' )? ')'
 ;



literal
 : NN
 | NUMBER
 | STRING
 | NULL
 ;


unaryOperator
 : '-'
 | '+'
 | '~'
 | '!'
 ;


ID: [a-zA-Z_][a-zA-Z0-9_]* ;
WS : [ \t\r\n]+ -> skip ;
LPAR : '(' ;
RPAR: ')';
LBRA: '[' ;
RBRA: ']';
COMMA: ',';
PIPE: '|' ;
COLON: ':';
NULL: 'null' | 'NULL';


NN: INTF; // NN: Natural Number {0,1,2,3, ...}

NUMBER
    : NN
    | '-'? INTF '.' [0-9]+ EXP? // 1.35, 1.35E-9, 0.3, -4.5
    |   '-'? INTF EXP             // 1e10 -3e4
    |   '-'? INTF                 // -3, 45
    ;
fragment INTF :   '0' | [1-9] [0-9]* ; // no leading zeros
fragment EXP :   [Ee] [+\-]? INTF ; // \- since - means "range" inside [...]


LT_EQ : '<=';
LT : '<';
GT_EQ : '>=';
GT : '>';
NEQ : '!=';
EQ : '==';

SEL: '.' ID ('.' ID)*; // SEL can be .THING or .THING.OTHERTHING etc.
DATASOURCE: '@' ID; // DS (Data Source): @mydb1 or @postgres_db2 etc.


STRING :  '"' (ESC | ~["\\])* '"' ;
fragment ESC :   '\\' (["\\/bfnrt] | UNICODE) ;
fragment UNICODE : 'u' HEX HEX HEX HEX ;
fragment HEX : [0-9a-fA-F] ;

//NUMERIC_LITERAL
// : DIGIT+ ( '.' DIGIT* )? ( E [-+]? DIGIT+ )?
// | '.' DIGIT+ ( E [-+]? DIGIT+ )?
// ;



fragment DIGIT : [0-9];

fragment A : [aA];
fragment B : [bB];
fragment C : [cC];
fragment D : [dD];
fragment E : [eE];
fragment F : [fF];
fragment G : [gG];
fragment H : [hH];
fragment I : [iI];
fragment J : [jJ];
fragment K : [kK];
fragment L : [lL];
fragment M : [mM];
fragment N : [nN];
fragment O : [oO];
fragment P : [pP];
fragment Q : [qQ];
fragment R : [rR];
fragment S : [sS];
fragment T : [tT];
fragment U : [uU];
fragment V : [vV];
fragment W : [wW];
fragment X : [xX];
fragment Y : [yY];
fragment Z : [zZ];

LINECOMMENT: '//' .*? '\n' -> skip ;