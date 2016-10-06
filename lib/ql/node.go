package ql

import (
	"fmt"

	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

type Node interface {
	Parent() Node
	SetParent(Node) error
	Children() []Node
	SetChildren([]Node) error
	AddChild(Node) error
	Context() antlr.ParseTree
	SetContext(antlr.ParseTree) error
	//Index() int
	String() string
	Text() string
	// Replace swaps the existing node with the provided node.
	//Replace(Node) error
	//Process() (bool, error)
	//Finished() bool

}

type BaseNode struct {
	parent   Node
	children []Node
	ctx      antlr.ParseTree
}

func (bn *BaseNode) Parent() Node {
	return bn.parent
}

func (bn *BaseNode) SetParent(parent Node) error {
	bn.parent = parent
	return nil
}

//
//func (bn *BaseNode) Index() int {
//	parent := bn.Parent()
//	if parent == nil {
//		return -1
//	}
//
//	for i, sibling := range parent.Children() {
//		if
//	}
//
//}

func (bn *BaseNode) Children() []Node {
	return bn.children
}

func (bn *BaseNode) AddChild(child Node) error {
	return errorf(emsgNodeNoAddChild, bn, child)
}

func (bn *BaseNode) addChild(child Node) {
	bn.children = append(bn.children, child)
}

func (bn *BaseNode) SetChildren(children []Node) error {

	return errorf(emsgNodeNoAddChildren, bn, len(children))
}

func (bn *BaseNode) setChildren(children []Node) {
	bn.children = children
}

func (bn *BaseNode) Text() string {
	if bn.ctx == nil {
		return ""
	}

	return bn.ctx.GetText()
}

func (bn *BaseNode) Context() antlr.ParseTree {
	return bn.ctx
}

func (bn *BaseNode) SetContext(ctx antlr.ParseTree) error {
	bn.ctx = ctx
	return nil
}

func ToTreeString(node Node) string {
	return nodeTreeString(node, "", 0)
}

func nodeTreeString(node Node, str string, depth int) string {

	//lg.Debugf("entering with node: %T", node)

	pad := strings.Repeat("    ", depth) + "- "

	//pad := fmt.Sprintf("%"+strconv.Itoa(depth)+"s ", "-")

	//nodeStr := str + fmt.Sprintf("%s %T %s", pad, node, node.String())
	nodeStr := fmt.Sprintf("%s(%T)  %s", pad, node, node.String())
	//childrenStr := ""

	cStr := []string{}

	for _, child := range node.Children() {
		//lg.Debugf("about to process child: %T", child)

		cStr = append(cStr, nodeTreeString(child, str, depth+1))
		//childrenStr = childrenStr + nodeTreeString(child, str, depth+1)
	}

	if len(cStr) > 0 {
		nodeStr = nodeStr + "\n" + strings.Join(cStr, "\n")
	}

	return nodeStr
}

// nodeString returns a default value suitable for use by Node.String().
func nodeString(n Node) string {
	return fmt.Sprintf("%T: %s", n, n.Text())
}

func ReplaceNode(old Node, nu Node) error {

	lg.Debugf("replacing node %T(%q) with %T(%q)", old, old.Text(), nu, nu.Text())

	err := nu.SetContext(old.Context())
	if err != nil {
		return err
	}

	parent := old.Parent()

	index := ChildIndex(parent, old)
	if index < 0 {
		return errorf("parent %T(%q) does not appear to have child %T(%q)", parent, parent.Text(), old, old.Text())
	}
	siblings := parent.Children()
	siblings[index] = nu

	return parent.SetChildren(siblings)
	//return errorf("not implemented")
}

func ChildIndex(parent Node, child Node) int {

	index := -1

	for i, chld := range parent.Children() {
		if chld == child {
			index = i
			break
		}
	}

	return index
}

//func (bn *BaseNode) Replace(swap Node) error {
//
//	//lg.Debugf("replacing %q with %q", bn.String(), swap.String())
//	//
//	swap.SetContext(bn.Context())
//
//	for _, child := range bn.Children() {
//		err := child.SetParent(swap)
//		if err != nil {
//			return err
//		}
//	}
//
//	siblings := bn.Parent().Children()
//
//	for i, node := range siblings {
//		if node == bn {
//			siblings[i] = swap
//			err := bn.Parent().SetChildren(siblings)
//			if err != nil {
//				return err
//			}
//			return nil
//		}
//	}
//
//	return newIRError("unable to find item in parent to swap")
//}
