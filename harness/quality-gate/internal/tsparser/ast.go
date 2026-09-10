package tsparser

// Kind mirrors the ts-morph SyntaxKinds the scanners branch on.
type Kind int

const (
	KUnknown Kind = iota
	KSourceFile
	KBlock
	KIfStatement
	KForStatement
	KForInStatement
	KForOfStatement
	KWhileStatement
	KDoStatement
	KSwitchStatement
	KCaseClause
	KDefaultClause
	KCatchClause
	KReturnStatement
	KVariableStatement
	KVariableDeclaration
	KFunctionDeclaration
	KMethodDeclaration
	KConstructor
	KGetAccessor
	KSetAccessor
	KImportDeclaration
	KExpressionStatement
	KIdentifier
	KNumericLiteral
	KStringLiteral
	KNoSubstitutionTemplateLiteral
	KTemplateExpression
	KNullKeyword
	KTrueKeyword
	KFalseKeyword
	KBinaryExpression
	KPrefixUnaryExpression
	KConditionalExpression
	KCallExpression
	KNewExpression
	KPropertyAccessExpression
	KElementAccessExpression
	KArrowFunction
	KFunctionExpression
	KObjectLiteralExpression
	KArrayLiteralExpression
	KParenthesizedExpression
	KAwaitExpression
	KSpreadElement
	KJsxElement
	KJsxSelfClosingElement
	KJsxFragment
	KJsxOpeningElement
	KJsxAttribute
	KJsxSpreadAttribute
	KJsxExpression
	KIndexSignature
	KArrayBindingPattern
	KBindingElement
	KParameter
	KPropertyAssignment
)

// Node is a lightweight AST node carrying the fields the scanners read.
type Node struct {
	Kind     Kind
	Line     int
	Text     string // raw source span
	Parent   *Node
	Children []*Node // structural children, in source order

	// Typed slots, populated per kind (nil when not applicable). Operands not
	// listed here stay reachable through Children, in source order.
	Name        string  // identifier text, property name, callee-ish name
	Expr        *Node   // primary sub-expression (condition, callee, member object, initializer target)
	Else        *Node   // IfStatement else branch
	Body        *Node   // function/loop body
	Args        []*Node // CallExpression arguments
	Params      []*Node // function parameters
	Elements    []*Node // array literal / binding-pattern / object-literal elements / attributes
	NameNode    *Node   // VariableDeclaration name node
	Initializer *Node   // VariableDeclaration initializer
}

func newNode(kind Kind, line int) *Node {
	return &Node{Kind: kind, Line: line}
}

func (n *Node) add(child *Node) *Node {
	if child == nil {
		return child
	}
	child.Parent = n
	n.Children = append(n.Children, child)
	return child
}

// walk visits every descendant depth-first, excluding n itself.
func (n *Node) walk(fn func(*Node)) {
	for _, c := range n.Children {
		if c == nil {
			continue
		}
		fn(c)
		c.walk(fn)
	}
}

// GetDescendantsOfKind collects all descendants of the given kind.
func (n *Node) GetDescendantsOfKind(k Kind) []*Node {
	var out []*Node
	n.walk(func(c *Node) {
		if c.Kind == k {
			out = append(out, c)
		}
	})
	return out
}
