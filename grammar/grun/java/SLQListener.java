// Generated from ../../grammar/SLQ.g4 by ANTLR 4.5.3
import org.antlr.v4.runtime.tree.ParseTreeListener;

/**
 * This interface defines a complete listener for a parse tree produced by
 * {@link SLQParser}.
 */
public interface SLQListener extends ParseTreeListener {
	/**
	 * Enter a parse tree produced by {@link SLQParser#stmtList}.
	 * @param ctx the parse tree
	 */
	void enterStmtList(SLQParser.StmtListContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#stmtList}.
	 * @param ctx the parse tree
	 */
	void exitStmtList(SLQParser.StmtListContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#query}.
	 * @param ctx the parse tree
	 */
	void enterQuery(SLQParser.QueryContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#query}.
	 * @param ctx the parse tree
	 */
	void exitQuery(SLQParser.QueryContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#segment}.
	 * @param ctx the parse tree
	 */
	void enterSegment(SLQParser.SegmentContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#segment}.
	 * @param ctx the parse tree
	 */
	void exitSegment(SLQParser.SegmentContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#element}.
	 * @param ctx the parse tree
	 */
	void enterElement(SLQParser.ElementContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#element}.
	 * @param ctx the parse tree
	 */
	void exitElement(SLQParser.ElementContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#cmpr}.
	 * @param ctx the parse tree
	 */
	void enterCmpr(SLQParser.CmprContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#cmpr}.
	 * @param ctx the parse tree
	 */
	void exitCmpr(SLQParser.CmprContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#fn}.
	 * @param ctx the parse tree
	 */
	void enterFn(SLQParser.FnContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#fn}.
	 * @param ctx the parse tree
	 */
	void exitFn(SLQParser.FnContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#join}.
	 * @param ctx the parse tree
	 */
	void enterJoin(SLQParser.JoinContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#join}.
	 * @param ctx the parse tree
	 */
	void exitJoin(SLQParser.JoinContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#joinConstraint}.
	 * @param ctx the parse tree
	 */
	void enterJoinConstraint(SLQParser.JoinConstraintContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#joinConstraint}.
	 * @param ctx the parse tree
	 */
	void exitJoinConstraint(SLQParser.JoinConstraintContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#selElement}.
	 * @param ctx the parse tree
	 */
	void enterSelElement(SLQParser.SelElementContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#selElement}.
	 * @param ctx the parse tree
	 */
	void exitSelElement(SLQParser.SelElementContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#dsTblElement}.
	 * @param ctx the parse tree
	 */
	void enterDsTblElement(SLQParser.DsTblElementContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#dsTblElement}.
	 * @param ctx the parse tree
	 */
	void exitDsTblElement(SLQParser.DsTblElementContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#dsElement}.
	 * @param ctx the parse tree
	 */
	void enterDsElement(SLQParser.DsElementContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#dsElement}.
	 * @param ctx the parse tree
	 */
	void exitDsElement(SLQParser.DsElementContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#rowRange}.
	 * @param ctx the parse tree
	 */
	void enterRowRange(SLQParser.RowRangeContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#rowRange}.
	 * @param ctx the parse tree
	 */
	void exitRowRange(SLQParser.RowRangeContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#fnName}.
	 * @param ctx the parse tree
	 */
	void enterFnName(SLQParser.FnNameContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#fnName}.
	 * @param ctx the parse tree
	 */
	void exitFnName(SLQParser.FnNameContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#expr}.
	 * @param ctx the parse tree
	 */
	void enterExpr(SLQParser.ExprContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#expr}.
	 * @param ctx the parse tree
	 */
	void exitExpr(SLQParser.ExprContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#literal}.
	 * @param ctx the parse tree
	 */
	void enterLiteral(SLQParser.LiteralContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#literal}.
	 * @param ctx the parse tree
	 */
	void exitLiteral(SLQParser.LiteralContext ctx);
	/**
	 * Enter a parse tree produced by {@link SLQParser#unaryOperator}.
	 * @param ctx the parse tree
	 */
	void enterUnaryOperator(SLQParser.UnaryOperatorContext ctx);
	/**
	 * Exit a parse tree produced by {@link SLQParser#unaryOperator}.
	 * @param ctx the parse tree
	 */
	void exitUnaryOperator(SLQParser.UnaryOperatorContext ctx);
}