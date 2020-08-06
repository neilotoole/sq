// Generated from ../../grammar/SLQ.g4 by ANTLR 4.5.3
import org.antlr.v4.runtime.atn.*;
import org.antlr.v4.runtime.dfa.DFA;
import org.antlr.v4.runtime.*;
import org.antlr.v4.runtime.misc.*;
import org.antlr.v4.runtime.tree.*;
import java.util.List;
import java.util.Iterator;
import java.util.ArrayList;

@SuppressWarnings({"all", "warnings", "unchecked", "unused", "cast"})
public class SLQParser extends Parser {
	static { RuntimeMetaData.checkVersion("4.5.3", RuntimeMetaData.VERSION); }

	protected static final DFA[] _decisionToDFA;
	protected static final PredictionContextCache _sharedContextCache =
		new PredictionContextCache();
	public static final int
		T__0=1, T__1=2, T__2=3, T__3=4, T__4=5, T__5=6, T__6=7, T__7=8, T__8=9, 
		T__9=10, T__10=11, T__11=12, T__12=13, T__13=14, T__14=15, T__15=16, T__16=17, 
		T__17=18, T__18=19, T__19=20, T__20=21, T__21=22, T__22=23, T__23=24, 
		T__24=25, ID=26, WS=27, LPAR=28, RPAR=29, LBRA=30, RBRA=31, COMMA=32, 
		PIPE=33, COLON=34, NULL=35, NN=36, NUMBER=37, LT_EQ=38, LT=39, GT_EQ=40, 
		GT=41, NEQ=42, EQ=43, SEL=44, DATASOURCE=45, STRING=46, LINECOMMENT=47;
	public static final int
		RULE_stmtList = 0, RULE_query = 1, RULE_segment = 2, RULE_element = 3, 
		RULE_cmpr = 4, RULE_fn = 5, RULE_join = 6, RULE_joinConstraint = 7, RULE_selElement = 8, 
		RULE_dsTblElement = 9, RULE_dsElement = 10, RULE_rowRange = 11, RULE_fnName = 12, 
		RULE_expr = 13, RULE_literal = 14, RULE_unaryOperator = 15;
	public static final String[] ruleNames = {
		"stmtList", "query", "segment", "element", "cmpr", "fn", "join", "joinConstraint", 
		"selElement", "dsTblElement", "dsElement", "rowRange", "fnName", "expr", 
		"literal", "unaryOperator"
	};

	private static final String[] _LITERAL_NAMES = {
		null, "';'", "'*'", "'join'", "'JOIN'", "'j'", "'.['", "'sum'", "'SUM'", 
		"'avg'", "'AVG'", "'count'", "'COUNT'", "'where'", "'WHERE'", "'||'", 
		"'/'", "'%'", "'+'", "'-'", "'<<'", "'>>'", "'&'", "'&&'", "'~'", "'!'", 
		null, null, "'('", "')'", "'['", "']'", "','", "'|'", "':'", null, null, 
		null, "'<='", "'<'", "'>='", "'>'", "'!='", "'=='"
	};
	private static final String[] _SYMBOLIC_NAMES = {
		null, null, null, null, null, null, null, null, null, null, null, null, 
		null, null, null, null, null, null, null, null, null, null, null, null, 
		null, null, "ID", "WS", "LPAR", "RPAR", "LBRA", "RBRA", "COMMA", "PIPE", 
		"COLON", "NULL", "NN", "NUMBER", "LT_EQ", "LT", "GT_EQ", "GT", "NEQ", 
		"EQ", "SEL", "DATASOURCE", "STRING", "LINECOMMENT"
	};
	public static final Vocabulary VOCABULARY = new VocabularyImpl(_LITERAL_NAMES, _SYMBOLIC_NAMES);

	/**
	 * @deprecated Use {@link #VOCABULARY} instead.
	 */
	@Deprecated
	public static final String[] tokenNames;
	static {
		tokenNames = new String[_SYMBOLIC_NAMES.length];
		for (int i = 0; i < tokenNames.length; i++) {
			tokenNames[i] = VOCABULARY.getLiteralName(i);
			if (tokenNames[i] == null) {
				tokenNames[i] = VOCABULARY.getSymbolicName(i);
			}

			if (tokenNames[i] == null) {
				tokenNames[i] = "<INVALID>";
			}
		}
	}

	@Override
	@Deprecated
	public String[] getTokenNames() {
		return tokenNames;
	}

	@Override

	public Vocabulary getVocabulary() {
		return VOCABULARY;
	}

	@Override
	public String getGrammarFileName() { return "SLQ.g4"; }

	@Override
	public String[] getRuleNames() { return ruleNames; }

	@Override
	public String getSerializedATN() { return _serializedATN; }

	@Override
	public ATN getATN() { return _ATN; }

	public SLQParser(TokenStream input) {
		super(input);
		_interp = new ParserATNSimulator(this,_ATN,_decisionToDFA,_sharedContextCache);
	}
	public static class StmtListContext extends ParserRuleContext {
		public List<QueryContext> query() {
			return getRuleContexts(QueryContext.class);
		}
		public QueryContext query(int i) {
			return getRuleContext(QueryContext.class,i);
		}
		public StmtListContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_stmtList; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterStmtList(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitStmtList(this);
		}
	}

	public final StmtListContext stmtList() throws RecognitionException {
		StmtListContext _localctx = new StmtListContext(_ctx, getState());
		enterRule(_localctx, 0, RULE_stmtList);
		int _la;
		try {
			int _alt;
			enterOuterAlt(_localctx, 1);
			{
			setState(35);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==T__0) {
				{
				{
				setState(32);
				match(T__0);
				}
				}
				setState(37);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			setState(38);
			query();
			setState(47);
			_errHandler.sync(this);
			_alt = getInterpreter().adaptivePredict(_input,2,_ctx);
			while ( _alt!=2 && _alt!=org.antlr.v4.runtime.atn.ATN.INVALID_ALT_NUMBER ) {
				if ( _alt==1 ) {
					{
					{
					setState(40); 
					_errHandler.sync(this);
					_la = _input.LA(1);
					do {
						{
						{
						setState(39);
						match(T__0);
						}
						}
						setState(42); 
						_errHandler.sync(this);
						_la = _input.LA(1);
					} while ( _la==T__0 );
					setState(44);
					query();
					}
					} 
				}
				setState(49);
				_errHandler.sync(this);
				_alt = getInterpreter().adaptivePredict(_input,2,_ctx);
			}
			setState(53);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==T__0) {
				{
				{
				setState(50);
				match(T__0);
				}
				}
				setState(55);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class QueryContext extends ParserRuleContext {
		public List<SegmentContext> segment() {
			return getRuleContexts(SegmentContext.class);
		}
		public SegmentContext segment(int i) {
			return getRuleContext(SegmentContext.class,i);
		}
		public QueryContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_query; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterQuery(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitQuery(this);
		}
	}

	public final QueryContext query() throws RecognitionException {
		QueryContext _localctx = new QueryContext(_ctx, getState());
		enterRule(_localctx, 2, RULE_query);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(56);
			segment();
			setState(61);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==PIPE) {
				{
				{
				setState(57);
				match(PIPE);
				setState(58);
				segment();
				}
				}
				setState(63);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class SegmentContext extends ParserRuleContext {
		public List<ElementContext> element() {
			return getRuleContexts(ElementContext.class);
		}
		public ElementContext element(int i) {
			return getRuleContext(ElementContext.class,i);
		}
		public SegmentContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_segment; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterSegment(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitSegment(this);
		}
	}

	public final SegmentContext segment() throws RecognitionException {
		SegmentContext _localctx = new SegmentContext(_ctx, getState());
		enterRule(_localctx, 4, RULE_segment);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			{
			setState(64);
			element();
			}
			setState(69);
			_errHandler.sync(this);
			_la = _input.LA(1);
			while (_la==COMMA) {
				{
				{
				setState(65);
				match(COMMA);
				setState(66);
				element();
				}
				}
				setState(71);
				_errHandler.sync(this);
				_la = _input.LA(1);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class ElementContext extends ParserRuleContext {
		public DsTblElementContext dsTblElement() {
			return getRuleContext(DsTblElementContext.class,0);
		}
		public DsElementContext dsElement() {
			return getRuleContext(DsElementContext.class,0);
		}
		public SelElementContext selElement() {
			return getRuleContext(SelElementContext.class,0);
		}
		public JoinContext join() {
			return getRuleContext(JoinContext.class,0);
		}
		public RowRangeContext rowRange() {
			return getRuleContext(RowRangeContext.class,0);
		}
		public ExprContext expr() {
			return getRuleContext(ExprContext.class,0);
		}
		public ElementContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_element; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterElement(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitElement(this);
		}
	}

	public final ElementContext element() throws RecognitionException {
		ElementContext _localctx = new ElementContext(_ctx, getState());
		enterRule(_localctx, 6, RULE_element);
		try {
			setState(78);
			_errHandler.sync(this);
			switch ( getInterpreter().adaptivePredict(_input,6,_ctx) ) {
			case 1:
				enterOuterAlt(_localctx, 1);
				{
				setState(72);
				dsTblElement();
				}
				break;
			case 2:
				enterOuterAlt(_localctx, 2);
				{
				setState(73);
				dsElement();
				}
				break;
			case 3:
				enterOuterAlt(_localctx, 3);
				{
				setState(74);
				selElement();
				}
				break;
			case 4:
				enterOuterAlt(_localctx, 4);
				{
				setState(75);
				join();
				}
				break;
			case 5:
				enterOuterAlt(_localctx, 5);
				{
				setState(76);
				rowRange();
				}
				break;
			case 6:
				enterOuterAlt(_localctx, 6);
				{
				setState(77);
				expr(0);
				}
				break;
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class CmprContext extends ParserRuleContext {
		public TerminalNode LT_EQ() { return getToken(SLQParser.LT_EQ, 0); }
		public TerminalNode LT() { return getToken(SLQParser.LT, 0); }
		public TerminalNode GT_EQ() { return getToken(SLQParser.GT_EQ, 0); }
		public TerminalNode GT() { return getToken(SLQParser.GT, 0); }
		public TerminalNode EQ() { return getToken(SLQParser.EQ, 0); }
		public TerminalNode NEQ() { return getToken(SLQParser.NEQ, 0); }
		public CmprContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_cmpr; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterCmpr(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitCmpr(this);
		}
	}

	public final CmprContext cmpr() throws RecognitionException {
		CmprContext _localctx = new CmprContext(_ctx, getState());
		enterRule(_localctx, 8, RULE_cmpr);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(80);
			_la = _input.LA(1);
			if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & ((1L << LT_EQ) | (1L << LT) | (1L << GT_EQ) | (1L << GT) | (1L << NEQ) | (1L << EQ))) != 0)) ) {
			_errHandler.recoverInline(this);
			} else {
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class FnContext extends ParserRuleContext {
		public FnNameContext fnName() {
			return getRuleContext(FnNameContext.class,0);
		}
		public List<ExprContext> expr() {
			return getRuleContexts(ExprContext.class);
		}
		public ExprContext expr(int i) {
			return getRuleContext(ExprContext.class,i);
		}
		public FnContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_fn; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterFn(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitFn(this);
		}
	}

	public final FnContext fn() throws RecognitionException {
		FnContext _localctx = new FnContext(_ctx, getState());
		enterRule(_localctx, 10, RULE_fn);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(82);
			fnName();
			setState(83);
			match(LPAR);
			setState(93);
			switch (_input.LA(1)) {
			case T__6:
			case T__7:
			case T__8:
			case T__9:
			case T__10:
			case T__11:
			case T__12:
			case T__13:
			case T__17:
			case T__18:
			case T__23:
			case T__24:
			case NULL:
			case NN:
			case NUMBER:
			case SEL:
			case STRING:
				{
				setState(84);
				expr(0);
				setState(89);
				_errHandler.sync(this);
				_la = _input.LA(1);
				while (_la==COMMA) {
					{
					{
					setState(85);
					match(COMMA);
					setState(86);
					expr(0);
					}
					}
					setState(91);
					_errHandler.sync(this);
					_la = _input.LA(1);
				}
				}
				break;
			case T__1:
				{
				setState(92);
				match(T__1);
				}
				break;
			case RPAR:
				break;
			default:
				throw new NoViableAltException(this);
			}
			setState(95);
			match(RPAR);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class JoinContext extends ParserRuleContext {
		public JoinConstraintContext joinConstraint() {
			return getRuleContext(JoinConstraintContext.class,0);
		}
		public JoinContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_join; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterJoin(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitJoin(this);
		}
	}

	public final JoinContext join() throws RecognitionException {
		JoinContext _localctx = new JoinContext(_ctx, getState());
		enterRule(_localctx, 12, RULE_join);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(97);
			_la = _input.LA(1);
			if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & ((1L << T__2) | (1L << T__3) | (1L << T__4))) != 0)) ) {
			_errHandler.recoverInline(this);
			} else {
				consume();
			}
			setState(98);
			match(LPAR);
			setState(99);
			joinConstraint();
			setState(100);
			match(RPAR);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class JoinConstraintContext extends ParserRuleContext {
		public List<TerminalNode> SEL() { return getTokens(SLQParser.SEL); }
		public TerminalNode SEL(int i) {
			return getToken(SLQParser.SEL, i);
		}
		public CmprContext cmpr() {
			return getRuleContext(CmprContext.class,0);
		}
		public JoinConstraintContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_joinConstraint; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterJoinConstraint(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitJoinConstraint(this);
		}
	}

	public final JoinConstraintContext joinConstraint() throws RecognitionException {
		JoinConstraintContext _localctx = new JoinConstraintContext(_ctx, getState());
		enterRule(_localctx, 14, RULE_joinConstraint);
		try {
			setState(107);
			_errHandler.sync(this);
			switch ( getInterpreter().adaptivePredict(_input,9,_ctx) ) {
			case 1:
				enterOuterAlt(_localctx, 1);
				{
				setState(102);
				match(SEL);
				setState(103);
				cmpr();
				setState(104);
				match(SEL);
				}
				break;
			case 2:
				enterOuterAlt(_localctx, 2);
				{
				setState(106);
				match(SEL);
				}
				break;
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class SelElementContext extends ParserRuleContext {
		public TerminalNode SEL() { return getToken(SLQParser.SEL, 0); }
		public SelElementContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_selElement; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterSelElement(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitSelElement(this);
		}
	}

	public final SelElementContext selElement() throws RecognitionException {
		SelElementContext _localctx = new SelElementContext(_ctx, getState());
		enterRule(_localctx, 16, RULE_selElement);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(109);
			match(SEL);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class DsTblElementContext extends ParserRuleContext {
		public TerminalNode DATASOURCE() { return getToken(SLQParser.DATASOURCE, 0); }
		public TerminalNode SEL() { return getToken(SLQParser.SEL, 0); }
		public DsTblElementContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_dsTblElement; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterDsTblElement(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitDsTblElement(this);
		}
	}

	public final DsTblElementContext dsTblElement() throws RecognitionException {
		DsTblElementContext _localctx = new DsTblElementContext(_ctx, getState());
		enterRule(_localctx, 18, RULE_dsTblElement);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(111);
			match(DATASOURCE);
			setState(112);
			match(SEL);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class DsElementContext extends ParserRuleContext {
		public TerminalNode DATASOURCE() { return getToken(SLQParser.DATASOURCE, 0); }
		public DsElementContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_dsElement; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterDsElement(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitDsElement(this);
		}
	}

	public final DsElementContext dsElement() throws RecognitionException {
		DsElementContext _localctx = new DsElementContext(_ctx, getState());
		enterRule(_localctx, 20, RULE_dsElement);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(114);
			match(DATASOURCE);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class RowRangeContext extends ParserRuleContext {
		public List<TerminalNode> NN() { return getTokens(SLQParser.NN); }
		public TerminalNode NN(int i) {
			return getToken(SLQParser.NN, i);
		}
		public TerminalNode COLON() { return getToken(SLQParser.COLON, 0); }
		public RowRangeContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_rowRange; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterRowRange(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitRowRange(this);
		}
	}

	public final RowRangeContext rowRange() throws RecognitionException {
		RowRangeContext _localctx = new RowRangeContext(_ctx, getState());
		enterRule(_localctx, 22, RULE_rowRange);
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(116);
			match(T__5);
			setState(125);
			_errHandler.sync(this);
			switch ( getInterpreter().adaptivePredict(_input,10,_ctx) ) {
			case 1:
				{
				setState(117);
				match(NN);
				setState(118);
				match(COLON);
				setState(119);
				match(NN);
				}
				break;
			case 2:
				{
				setState(120);
				match(NN);
				setState(121);
				match(COLON);
				}
				break;
			case 3:
				{
				setState(122);
				match(COLON);
				setState(123);
				match(NN);
				}
				break;
			case 4:
				{
				setState(124);
				match(NN);
				}
				break;
			}
			setState(127);
			match(RBRA);
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class FnNameContext extends ParserRuleContext {
		public FnNameContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_fnName; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterFnName(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitFnName(this);
		}
	}

	public final FnNameContext fnName() throws RecognitionException {
		FnNameContext _localctx = new FnNameContext(_ctx, getState());
		enterRule(_localctx, 24, RULE_fnName);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(129);
			_la = _input.LA(1);
			if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & ((1L << T__6) | (1L << T__7) | (1L << T__8) | (1L << T__9) | (1L << T__10) | (1L << T__11) | (1L << T__12) | (1L << T__13))) != 0)) ) {
			_errHandler.recoverInline(this);
			} else {
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class ExprContext extends ParserRuleContext {
		public TerminalNode SEL() { return getToken(SLQParser.SEL, 0); }
		public LiteralContext literal() {
			return getRuleContext(LiteralContext.class,0);
		}
		public UnaryOperatorContext unaryOperator() {
			return getRuleContext(UnaryOperatorContext.class,0);
		}
		public List<ExprContext> expr() {
			return getRuleContexts(ExprContext.class);
		}
		public ExprContext expr(int i) {
			return getRuleContext(ExprContext.class,i);
		}
		public FnNameContext fnName() {
			return getRuleContext(FnNameContext.class,0);
		}
		public ExprContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_expr; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterExpr(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitExpr(this);
		}
	}

	public final ExprContext expr() throws RecognitionException {
		return expr(0);
	}

	private ExprContext expr(int _p) throws RecognitionException {
		ParserRuleContext _parentctx = _ctx;
		int _parentState = getState();
		ExprContext _localctx = new ExprContext(_ctx, _parentState);
		ExprContext _prevctx = _localctx;
		int _startState = 26;
		enterRecursionRule(_localctx, 26, RULE_expr, _p);
		int _la;
		try {
			int _alt;
			enterOuterAlt(_localctx, 1);
			{
			setState(152);
			switch (_input.LA(1)) {
			case SEL:
				{
				setState(132);
				match(SEL);
				}
				break;
			case NULL:
			case NN:
			case NUMBER:
			case STRING:
				{
				setState(133);
				literal();
				}
				break;
			case T__17:
			case T__18:
			case T__23:
			case T__24:
				{
				setState(134);
				unaryOperator();
				setState(135);
				expr(9);
				}
				break;
			case T__6:
			case T__7:
			case T__8:
			case T__9:
			case T__10:
			case T__11:
			case T__12:
			case T__13:
				{
				setState(137);
				fnName();
				setState(138);
				match(LPAR);
				setState(148);
				switch (_input.LA(1)) {
				case T__6:
				case T__7:
				case T__8:
				case T__9:
				case T__10:
				case T__11:
				case T__12:
				case T__13:
				case T__17:
				case T__18:
				case T__23:
				case T__24:
				case NULL:
				case NN:
				case NUMBER:
				case SEL:
				case STRING:
					{
					setState(139);
					expr(0);
					setState(144);
					_errHandler.sync(this);
					_la = _input.LA(1);
					while (_la==COMMA) {
						{
						{
						setState(140);
						match(COMMA);
						setState(141);
						expr(0);
						}
						}
						setState(146);
						_errHandler.sync(this);
						_la = _input.LA(1);
					}
					}
					break;
				case T__1:
					{
					setState(147);
					match(T__1);
					}
					break;
				case RPAR:
					break;
				default:
					throw new NoViableAltException(this);
				}
				setState(150);
				match(RPAR);
				}
				break;
			default:
				throw new NoViableAltException(this);
			}
			_ctx.stop = _input.LT(-1);
			setState(181);
			_errHandler.sync(this);
			_alt = getInterpreter().adaptivePredict(_input,16,_ctx);
			while ( _alt!=2 && _alt!=org.antlr.v4.runtime.atn.ATN.INVALID_ALT_NUMBER ) {
				if ( _alt==1 ) {
					if ( _parseListeners!=null ) triggerExitRuleEvent();
					_prevctx = _localctx;
					{
					setState(179);
					_errHandler.sync(this);
					switch ( getInterpreter().adaptivePredict(_input,15,_ctx) ) {
					case 1:
						{
						_localctx = new ExprContext(_parentctx, _parentState);
						pushNewRecursionContext(_localctx, _startState, RULE_expr);
						setState(154);
						if (!(precpred(_ctx, 8))) throw new FailedPredicateException(this, "precpred(_ctx, 8)");
						setState(155);
						match(T__14);
						setState(156);
						expr(9);
						}
						break;
					case 2:
						{
						_localctx = new ExprContext(_parentctx, _parentState);
						pushNewRecursionContext(_localctx, _startState, RULE_expr);
						setState(157);
						if (!(precpred(_ctx, 7))) throw new FailedPredicateException(this, "precpred(_ctx, 7)");
						setState(158);
						_la = _input.LA(1);
						if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & ((1L << T__1) | (1L << T__15) | (1L << T__16))) != 0)) ) {
						_errHandler.recoverInline(this);
						} else {
							consume();
						}
						setState(159);
						expr(8);
						}
						break;
					case 3:
						{
						_localctx = new ExprContext(_parentctx, _parentState);
						pushNewRecursionContext(_localctx, _startState, RULE_expr);
						setState(160);
						if (!(precpred(_ctx, 6))) throw new FailedPredicateException(this, "precpred(_ctx, 6)");
						setState(161);
						_la = _input.LA(1);
						if ( !(_la==T__17 || _la==T__18) ) {
						_errHandler.recoverInline(this);
						} else {
							consume();
						}
						setState(162);
						expr(7);
						}
						break;
					case 4:
						{
						_localctx = new ExprContext(_parentctx, _parentState);
						pushNewRecursionContext(_localctx, _startState, RULE_expr);
						setState(163);
						if (!(precpred(_ctx, 5))) throw new FailedPredicateException(this, "precpred(_ctx, 5)");
						setState(164);
						_la = _input.LA(1);
						if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & ((1L << T__19) | (1L << T__20) | (1L << T__21))) != 0)) ) {
						_errHandler.recoverInline(this);
						} else {
							consume();
						}
						setState(165);
						expr(6);
						}
						break;
					case 5:
						{
						_localctx = new ExprContext(_parentctx, _parentState);
						pushNewRecursionContext(_localctx, _startState, RULE_expr);
						setState(166);
						if (!(precpred(_ctx, 4))) throw new FailedPredicateException(this, "precpred(_ctx, 4)");
						setState(167);
						_la = _input.LA(1);
						if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & ((1L << LT_EQ) | (1L << LT) | (1L << GT_EQ) | (1L << GT))) != 0)) ) {
						_errHandler.recoverInline(this);
						} else {
							consume();
						}
						setState(168);
						expr(5);
						}
						break;
					case 6:
						{
						_localctx = new ExprContext(_parentctx, _parentState);
						pushNewRecursionContext(_localctx, _startState, RULE_expr);
						setState(169);
						if (!(precpred(_ctx, 3))) throw new FailedPredicateException(this, "precpred(_ctx, 3)");
						setState(173);
						switch (_input.LA(1)) {
						case EQ:
							{
							setState(170);
							match(EQ);
							}
							break;
						case NEQ:
							{
							setState(171);
							match(NEQ);
							}
							break;
						case T__6:
						case T__7:
						case T__8:
						case T__9:
						case T__10:
						case T__11:
						case T__12:
						case T__13:
						case T__17:
						case T__18:
						case T__23:
						case T__24:
						case NULL:
						case NN:
						case NUMBER:
						case SEL:
						case STRING:
							{
							}
							break;
						default:
							throw new NoViableAltException(this);
						}
						setState(175);
						expr(4);
						}
						break;
					case 7:
						{
						_localctx = new ExprContext(_parentctx, _parentState);
						pushNewRecursionContext(_localctx, _startState, RULE_expr);
						setState(176);
						if (!(precpred(_ctx, 2))) throw new FailedPredicateException(this, "precpred(_ctx, 2)");
						setState(177);
						match(T__22);
						setState(178);
						expr(3);
						}
						break;
					}
					} 
				}
				setState(183);
				_errHandler.sync(this);
				_alt = getInterpreter().adaptivePredict(_input,16,_ctx);
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			unrollRecursionContexts(_parentctx);
		}
		return _localctx;
	}

	public static class LiteralContext extends ParserRuleContext {
		public TerminalNode NN() { return getToken(SLQParser.NN, 0); }
		public TerminalNode NUMBER() { return getToken(SLQParser.NUMBER, 0); }
		public TerminalNode STRING() { return getToken(SLQParser.STRING, 0); }
		public TerminalNode NULL() { return getToken(SLQParser.NULL, 0); }
		public LiteralContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_literal; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterLiteral(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitLiteral(this);
		}
	}

	public final LiteralContext literal() throws RecognitionException {
		LiteralContext _localctx = new LiteralContext(_ctx, getState());
		enterRule(_localctx, 28, RULE_literal);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(184);
			_la = _input.LA(1);
			if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & ((1L << NULL) | (1L << NN) | (1L << NUMBER) | (1L << STRING))) != 0)) ) {
			_errHandler.recoverInline(this);
			} else {
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public static class UnaryOperatorContext extends ParserRuleContext {
		public UnaryOperatorContext(ParserRuleContext parent, int invokingState) {
			super(parent, invokingState);
		}
		@Override public int getRuleIndex() { return RULE_unaryOperator; }
		@Override
		public void enterRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).enterUnaryOperator(this);
		}
		@Override
		public void exitRule(ParseTreeListener listener) {
			if ( listener instanceof SLQListener ) ((SLQListener)listener).exitUnaryOperator(this);
		}
	}

	public final UnaryOperatorContext unaryOperator() throws RecognitionException {
		UnaryOperatorContext _localctx = new UnaryOperatorContext(_ctx, getState());
		enterRule(_localctx, 30, RULE_unaryOperator);
		int _la;
		try {
			enterOuterAlt(_localctx, 1);
			{
			setState(186);
			_la = _input.LA(1);
			if ( !((((_la) & ~0x3f) == 0 && ((1L << _la) & ((1L << T__17) | (1L << T__18) | (1L << T__23) | (1L << T__24))) != 0)) ) {
			_errHandler.recoverInline(this);
			} else {
				consume();
			}
			}
		}
		catch (RecognitionException re) {
			_localctx.exception = re;
			_errHandler.reportError(this, re);
			_errHandler.recover(this, re);
		}
		finally {
			exitRule();
		}
		return _localctx;
	}

	public boolean sempred(RuleContext _localctx, int ruleIndex, int predIndex) {
		switch (ruleIndex) {
		case 13:
			return expr_sempred((ExprContext)_localctx, predIndex);
		}
		return true;
	}
	private boolean expr_sempred(ExprContext _localctx, int predIndex) {
		switch (predIndex) {
		case 0:
			return precpred(_ctx, 8);
		case 1:
			return precpred(_ctx, 7);
		case 2:
			return precpred(_ctx, 6);
		case 3:
			return precpred(_ctx, 5);
		case 4:
			return precpred(_ctx, 4);
		case 5:
			return precpred(_ctx, 3);
		case 6:
			return precpred(_ctx, 2);
		}
		return true;
	}

	public static final String _serializedATN =
		"\3\u0430\ud6d1\u8206\uad2d\u4417\uaef1\u8d80\uaadd\3\61\u00bf\4\2\t\2"+
		"\4\3\t\3\4\4\t\4\4\5\t\5\4\6\t\6\4\7\t\7\4\b\t\b\4\t\t\t\4\n\t\n\4\13"+
		"\t\13\4\f\t\f\4\r\t\r\4\16\t\16\4\17\t\17\4\20\t\20\4\21\t\21\3\2\7\2"+
		"$\n\2\f\2\16\2\'\13\2\3\2\3\2\6\2+\n\2\r\2\16\2,\3\2\7\2\60\n\2\f\2\16"+
		"\2\63\13\2\3\2\7\2\66\n\2\f\2\16\29\13\2\3\3\3\3\3\3\7\3>\n\3\f\3\16\3"+
		"A\13\3\3\4\3\4\3\4\7\4F\n\4\f\4\16\4I\13\4\3\5\3\5\3\5\3\5\3\5\3\5\5\5"+
		"Q\n\5\3\6\3\6\3\7\3\7\3\7\3\7\3\7\7\7Z\n\7\f\7\16\7]\13\7\3\7\5\7`\n\7"+
		"\3\7\3\7\3\b\3\b\3\b\3\b\3\b\3\t\3\t\3\t\3\t\3\t\5\tn\n\t\3\n\3\n\3\13"+
		"\3\13\3\13\3\f\3\f\3\r\3\r\3\r\3\r\3\r\3\r\3\r\3\r\3\r\5\r\u0080\n\r\3"+
		"\r\3\r\3\16\3\16\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17"+
		"\7\17\u0091\n\17\f\17\16\17\u0094\13\17\3\17\5\17\u0097\n\17\3\17\3\17"+
		"\5\17\u009b\n\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17"+
		"\3\17\3\17\3\17\3\17\3\17\3\17\3\17\3\17\5\17\u00b0\n\17\3\17\3\17\3\17"+
		"\3\17\7\17\u00b6\n\17\f\17\16\17\u00b9\13\17\3\20\3\20\3\21\3\21\3\21"+
		"\2\3\34\22\2\4\6\b\n\f\16\20\22\24\26\30\32\34\36 \2\13\3\2(-\3\2\5\7"+
		"\3\2\t\20\4\2\4\4\22\23\3\2\24\25\3\2\26\30\3\2(+\4\2%\'\60\60\4\2\24"+
		"\25\32\33\u00d0\2%\3\2\2\2\4:\3\2\2\2\6B\3\2\2\2\bP\3\2\2\2\nR\3\2\2\2"+
		"\fT\3\2\2\2\16c\3\2\2\2\20m\3\2\2\2\22o\3\2\2\2\24q\3\2\2\2\26t\3\2\2"+
		"\2\30v\3\2\2\2\32\u0083\3\2\2\2\34\u009a\3\2\2\2\36\u00ba\3\2\2\2 \u00bc"+
		"\3\2\2\2\"$\7\3\2\2#\"\3\2\2\2$\'\3\2\2\2%#\3\2\2\2%&\3\2\2\2&(\3\2\2"+
		"\2\'%\3\2\2\2(\61\5\4\3\2)+\7\3\2\2*)\3\2\2\2+,\3\2\2\2,*\3\2\2\2,-\3"+
		"\2\2\2-.\3\2\2\2.\60\5\4\3\2/*\3\2\2\2\60\63\3\2\2\2\61/\3\2\2\2\61\62"+
		"\3\2\2\2\62\67\3\2\2\2\63\61\3\2\2\2\64\66\7\3\2\2\65\64\3\2\2\2\669\3"+
		"\2\2\2\67\65\3\2\2\2\678\3\2\2\28\3\3\2\2\29\67\3\2\2\2:?\5\6\4\2;<\7"+
		"#\2\2<>\5\6\4\2=;\3\2\2\2>A\3\2\2\2?=\3\2\2\2?@\3\2\2\2@\5\3\2\2\2A?\3"+
		"\2\2\2BG\5\b\5\2CD\7\"\2\2DF\5\b\5\2EC\3\2\2\2FI\3\2\2\2GE\3\2\2\2GH\3"+
		"\2\2\2H\7\3\2\2\2IG\3\2\2\2JQ\5\24\13\2KQ\5\26\f\2LQ\5\22\n\2MQ\5\16\b"+
		"\2NQ\5\30\r\2OQ\5\34\17\2PJ\3\2\2\2PK\3\2\2\2PL\3\2\2\2PM\3\2\2\2PN\3"+
		"\2\2\2PO\3\2\2\2Q\t\3\2\2\2RS\t\2\2\2S\13\3\2\2\2TU\5\32\16\2U_\7\36\2"+
		"\2V[\5\34\17\2WX\7\"\2\2XZ\5\34\17\2YW\3\2\2\2Z]\3\2\2\2[Y\3\2\2\2[\\"+
		"\3\2\2\2\\`\3\2\2\2][\3\2\2\2^`\7\4\2\2_V\3\2\2\2_^\3\2\2\2_`\3\2\2\2"+
		"`a\3\2\2\2ab\7\37\2\2b\r\3\2\2\2cd\t\3\2\2de\7\36\2\2ef\5\20\t\2fg\7\37"+
		"\2\2g\17\3\2\2\2hi\7.\2\2ij\5\n\6\2jk\7.\2\2kn\3\2\2\2ln\7.\2\2mh\3\2"+
		"\2\2ml\3\2\2\2n\21\3\2\2\2op\7.\2\2p\23\3\2\2\2qr\7/\2\2rs\7.\2\2s\25"+
		"\3\2\2\2tu\7/\2\2u\27\3\2\2\2v\177\7\b\2\2wx\7&\2\2xy\7$\2\2y\u0080\7"+
		"&\2\2z{\7&\2\2{\u0080\7$\2\2|}\7$\2\2}\u0080\7&\2\2~\u0080\7&\2\2\177"+
		"w\3\2\2\2\177z\3\2\2\2\177|\3\2\2\2\177~\3\2\2\2\177\u0080\3\2\2\2\u0080"+
		"\u0081\3\2\2\2\u0081\u0082\7!\2\2\u0082\31\3\2\2\2\u0083\u0084\t\4\2\2"+
		"\u0084\33\3\2\2\2\u0085\u0086\b\17\1\2\u0086\u009b\7.\2\2\u0087\u009b"+
		"\5\36\20\2\u0088\u0089\5 \21\2\u0089\u008a\5\34\17\13\u008a\u009b\3\2"+
		"\2\2\u008b\u008c\5\32\16\2\u008c\u0096\7\36\2\2\u008d\u0092\5\34\17\2"+
		"\u008e\u008f\7\"\2\2\u008f\u0091\5\34\17\2\u0090\u008e\3\2\2\2\u0091\u0094"+
		"\3\2\2\2\u0092\u0090\3\2\2\2\u0092\u0093\3\2\2\2\u0093\u0097\3\2\2\2\u0094"+
		"\u0092\3\2\2\2\u0095\u0097\7\4\2\2\u0096\u008d\3\2\2\2\u0096\u0095\3\2"+
		"\2\2\u0096\u0097\3\2\2\2\u0097\u0098\3\2\2\2\u0098\u0099\7\37\2\2\u0099"+
		"\u009b\3\2\2\2\u009a\u0085\3\2\2\2\u009a\u0087\3\2\2\2\u009a\u0088\3\2"+
		"\2\2\u009a\u008b\3\2\2\2\u009b\u00b7\3\2\2\2\u009c\u009d\f\n\2\2\u009d"+
		"\u009e\7\21\2\2\u009e\u00b6\5\34\17\13\u009f\u00a0\f\t\2\2\u00a0\u00a1"+
		"\t\5\2\2\u00a1\u00b6\5\34\17\n\u00a2\u00a3\f\b\2\2\u00a3\u00a4\t\6\2\2"+
		"\u00a4\u00b6\5\34\17\t\u00a5\u00a6\f\7\2\2\u00a6\u00a7\t\7\2\2\u00a7\u00b6"+
		"\5\34\17\b\u00a8\u00a9\f\6\2\2\u00a9\u00aa\t\b\2\2\u00aa\u00b6\5\34\17"+
		"\7\u00ab\u00af\f\5\2\2\u00ac\u00b0\7-\2\2\u00ad\u00b0\7,\2\2\u00ae\u00b0"+
		"\3\2\2\2\u00af\u00ac\3\2\2\2\u00af\u00ad\3\2\2\2\u00af\u00ae\3\2\2\2\u00b0"+
		"\u00b1\3\2\2\2\u00b1\u00b6\5\34\17\6\u00b2\u00b3\f\4\2\2\u00b3\u00b4\7"+
		"\31\2\2\u00b4\u00b6\5\34\17\5\u00b5\u009c\3\2\2\2\u00b5\u009f\3\2\2\2"+
		"\u00b5\u00a2\3\2\2\2\u00b5\u00a5\3\2\2\2\u00b5\u00a8\3\2\2\2\u00b5\u00ab"+
		"\3\2\2\2\u00b5\u00b2\3\2\2\2\u00b6\u00b9\3\2\2\2\u00b7\u00b5\3\2\2\2\u00b7"+
		"\u00b8\3\2\2\2\u00b8\35\3\2\2\2\u00b9\u00b7\3\2\2\2\u00ba\u00bb\t\t\2"+
		"\2\u00bb\37\3\2\2\2\u00bc\u00bd\t\n\2\2\u00bd!\3\2\2\2\23%,\61\67?GP["+
		"_m\177\u0092\u0096\u009a\u00af\u00b5\u00b7";
	public static final ATN _ATN =
		new ATNDeserializer().deserialize(_serializedATN.toCharArray());
	static {
		_decisionToDFA = new DFA[_ATN.getNumberOfDecisions()];
		for (int i = 0; i < _ATN.getNumberOfDecisions(); i++) {
			_decisionToDFA[i] = new DFA(_ATN.getDecisionState(i), i);
		}
	}
}