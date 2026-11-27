package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestGORMQueriesUseStructuredClauses(t *testing.T) {
	t.Helper()

	var violations []string
	walkErr := filepath.WalkDir(repoPath("."), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipGORMContractDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".sql" {
			violations = append(violations, path+": SQL source file")
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, ".pb.go") {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		fileViolations, parseErr := inspectGORMContract(path, source)
		if parseErr != nil {
			return parseErr
		}
		violations = append(violations, fileViolations...)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk source tree: %v", walkErr)
	}
	if len(violations) > 0 {
		t.Fatalf("GORM contract violations:\n%s", strings.Join(violations, "\n"))
	}
}

func inspectGORMContract(path string, source []byte) ([]string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	ormImports := make(map[string]bool)
	var violations []string
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			return nil, err
		}
		switch importPath {
		case "database/sql", "database/sql/driver", "github.com/glebarez/go-sqlite", "modernc.org/sqlite", "github.com/mattn/go-sqlite3":
			violations = append(violations, fileSet.Position(importSpec.Pos()).String()+": direct database API import")
		}
		if importPath != "gorm.io/gorm" && importPath != "gorm.io/gorm/clause" {
			continue
		}
		alias := filepath.Base(importPath)
		if importSpec.Name != nil {
			alias = importSpec.Name.Name
		}
		if alias == "." {
			violations = append(violations, fileSet.Position(importSpec.Pos()).String()+": ORM dot import hides query expression ownership")
		}
		ormImports[alias] = true
	}
	statementPattern := regexp.MustCompile(`(?i)^\s*(select\s+.+\s+from\b|insert\s+into\b|update\s+\S+\s+set\b|delete\s+from\b|pragma\s+|alter\s+table\b|create\s+(table|index)\b|drop\s+(table|index)\b)`)
	report := func(node ast.Node, reason string) {
		violations = append(violations, fileSet.Position(node.Pos()).String()+": "+reason)
	}
	visitor := gormContractVisitor{scope: &gormContractScope{bindings: make(map[string]ast.Expr)}}
	visitor.inspect = func(node ast.Node, scope *gormContractScope) {
		switch expression := node.(type) {
		case *ast.BasicLit:
			if expression.Kind == token.STRING {
				value, _ := strconv.Unquote(expression.Value)
				if statementPattern.MatchString(value) {
					report(expression, "application-authored SQL statement")
				}
			}
		case *ast.SelectorExpr:
			switch expression.Sel.Name {
			case "Raw", "Exec", "ExecContext", "QueryContext", "QueryRow", "QueryRowContext", "Prepare", "PrepareContext":
				report(expression, "direct database statement API")
			case "Query":
				if scope.isDirectDatabaseHandle(expression.X, make(map[ast.Expr]bool)) {
					report(expression, "direct database statement API")
				}
			}
			packageName, isPackage := expression.X.(*ast.Ident)
			if isPackage && ormImports[packageName.Name] && (expression.Sel.Name == "Expr" || expression.Sel.Name == "NamedExpr") {
				report(expression, "raw expression clause")
			}
		case *ast.CompositeLit:
			selector, ok := expression.Type.(*ast.SelectorExpr)
			if !ok {
				break
			}
			packageName, isPackage := selector.X.(*ast.Ident)
			if !isPackage || !ormImports[packageName.Name] || (selector.Sel.Name != "Column" && selector.Sel.Name != "Table") {
				break
			}
			for _, element := range expression.Elts {
				field, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				name, ok := field.Key.(*ast.Ident)
				value, isLiteral := field.Value.(*ast.Ident)
				if ok && name.Name == "Raw" && (!isLiteral || value.Name != "false") {
					report(field, "unquoted query identifier")
				}
			}
		case *ast.CallExpr:
			selector, ok := expression.Fun.(*ast.SelectorExpr)
			if !ok {
				break
			}
			switch selector.Sel.Name {
			case "Where", "Or", "Not", "Order", "Joins", "InnerJoins", "Select", "Table", "Having":
				if len(expression.Args) > 0 && scope.isStringExpression(expression.Args[0], make(map[ast.Expr]bool)) {
					report(expression, "string-fragment query builder")
				}
			case "First", "Find", "Take", "Last", "Preload":
				if len(expression.Args) > 1 && scope.isStringExpression(expression.Args[1], make(map[ast.Expr]bool)) {
					report(expression, "string-fragment query condition")
				}
			}
		}
	}
	ast.Walk(visitor, file)
	return violations, nil
}

type gormContractScope struct {
	parent   *gormContractScope
	bindings map[string]ast.Expr
}

func (scope *gormContractScope) resolve(name string) (*gormContractScope, ast.Expr) {
	for current := scope; current != nil; current = current.parent {
		if expression, exists := current.bindings[name]; exists {
			return current, expression
		}
	}
	return nil, nil
}

type gormContractVisitor struct {
	scope   *gormContractScope
	inspect func(ast.Node, *gormContractScope)
}

func (visitor gormContractVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	switch declaration := node.(type) {
	case *ast.File:
		for _, declaration := range declaration.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				visitor.scope.bindings[function.Name.Name] = function.Type
			}
		}
	case *ast.FuncDecl, *ast.FuncLit, *ast.BlockStmt, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.CaseClause, *ast.CommClause, *ast.StructType, *ast.InterfaceType:
		visitor.scope = &gormContractScope{parent: visitor.scope, bindings: make(map[string]ast.Expr)}
	case *ast.Field:
		for _, name := range declaration.Names {
			visitor.scope.bindings[name.Name] = declaration.Type
		}
	case *ast.ValueSpec:
		for index, name := range declaration.Names {
			expression := declaration.Type
			if index < len(declaration.Values) {
				expression = declaration.Values[index]
			}
			visitor.scope.bindings[name.Name] = expression
		}
	case *ast.AssignStmt:
		for index, target := range declaration.Lhs {
			if name, ok := target.(*ast.Ident); ok && index < len(declaration.Rhs) {
				visitor.scope.bindings[name.Name] = declaration.Rhs[index]
			}
		}
	}
	visitor.inspect(node, visitor.scope)
	return visitor
}

func (scope *gormContractScope) isDirectDatabaseHandle(expression ast.Expr, visited map[ast.Expr]bool) bool {
	if expression == nil || visited[expression] {
		return false
	}
	visited[expression] = true
	switch value := expression.(type) {
	case *ast.SelectorExpr:
		return value.Sel.Name == "ConnPool"
	case *ast.CallExpr:
		selector, ok := value.Fun.(*ast.SelectorExpr)
		return ok && selector.Sel.Name == "DB"
	case *ast.Ident:
		bindingScope, binding := scope.resolve(value.Name)
		return bindingScope.isDirectDatabaseHandle(binding, visited)
	}
	return false
}

func (scope *gormContractScope) isStringExpression(expression ast.Expr, visited map[ast.Expr]bool) bool {
	if expression == nil || visited[expression] {
		return false
	}
	visited[expression] = true
	switch value := expression.(type) {
	case *ast.BasicLit:
		return value.Kind == token.STRING
	case *ast.ParenExpr:
		return scope.isStringExpression(value.X, visited)
	case *ast.BinaryExpr:
		return scope.isStringExpression(value.X, visited) || scope.isStringExpression(value.Y, visited)
	case *ast.Ident:
		if value.Name == "string" {
			return true
		}
		bindingScope, binding := scope.resolve(value.Name)
		return bindingScope.isStringExpression(binding, visited)
	case *ast.CallExpr:
		if identifier, ok := value.Fun.(*ast.Ident); ok {
			if identifier.Name == "string" {
				return true
			}
			_, binding := scope.resolve(identifier.Name)
			if function, ok := binding.(*ast.FuncType); ok && function.Results != nil && len(function.Results.List) == 1 {
				typeName, ok := function.Results.List[0].Type.(*ast.Ident)
				return ok && typeName.Name == "string"
			}
		}
		selector, ok := value.Fun.(*ast.SelectorExpr)
		return ok && (selector.Sel.Name == "Sprintf" || selector.Sel.Name == "Join" || selector.Sel.Name == "String")
	}
	return false
}

func TestGORMContractDetectsStatementBypasses(t *testing.T) {
	testCases := []struct {
		name string
		code string
	}{
		{name: "query row", code: `database.QueryRow("configuration")`},
		{name: "query method reference", code: `query := database.QueryRow; query("configuration")`},
		{name: "connection pool", code: `database.Statement.ConnPool.ExecContext(ctx, "statement")`},
		{name: "connection pool query", code: `database.Statement.ConnPool.Query("statement")`},
		{name: "lifecycle handle query", code: `handle, err := database.DB(); handle.Query("statement")`},
		{name: "backtick condition", code: "database.Where(`condition`)"},
		{name: "constant condition", code: `const condition = "condition"; database.Where(condition)`},
		{name: "variable condition", code: `condition := "condition"; database.Where(condition)`},
		{name: "nested string condition", code: `condition := clauses.Eq{Column: "id", Value: id}; { condition := "condition"; database.Where(condition) }`},
		{name: "string alias across shadow", code: `condition := "condition"; alias := condition; { condition := clauses.Eq{Column: "id", Value: id}; database.Where(alias) }`},
		{name: "formatted condition", code: `database.Where(fmt.Sprintf("condition"))`},
		{name: "inline condition", code: `database.First(&record, "condition")`},
		{name: "preload condition", code: `database.Preload("Recipients", "condition")`},
		{name: "aliased expression", code: `database.Where(clauses.Expr{SQL: "condition"})`},
		{name: "aliased named expression", code: `database.Where(clauses.NamedExpr{SQL: "condition"})`},
		{name: "raw column", code: `database.Order(clauses.Column{Name: "condition", Raw: true})`},
		{name: "raw table", code: `database.Table(clauses.Table{Name: "condition", Raw: true})`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := []byte("package fixture\nimport clauses \"gorm.io/gorm/clause\"\nfunc fixture() { " + testCase.code + " }")
			violations, err := inspectGORMContract("fixture.go", source)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if len(violations) == 0 {
				t.Fatal("expected a GORM contract violation")
			}
		})
	}
	for _, importPath := range []string{"database/sql", "database/sql/driver", "github.com/glebarez/go-sqlite", "modernc.org/sqlite"} {
		t.Run(importPath, func(t *testing.T) {
			source := []byte("package fixture\nimport \"" + importPath + "\"")
			violations, err := inspectGORMContract("fixture.go", source)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if len(violations) == 0 {
				t.Fatal("expected a direct database import violation")
			}
		})
	}
}

func TestGORMContractAcceptsStructuredQueriesAndLifecycleHandles(t *testing.T) {
	source := []byte(`package fixture
import "gorm.io/gorm/clause"
func fixture() {
    database.Where(&Identity{ID: identityID}).First(&record)
    database.Where(map[string]interface{}{"id": identityID})
    database.Order(clause.OrderByColumn{Column: clause.Column{Name: "created_at"}})
    database.Preload("Recipients").Find(&records)
    database.Migrator().HasTable(&Identity{})
    database.AutoMigrate(&Identity{})
    database.Callback().Query().Before("gorm:query")
    contextGin.Query("search")
    handle, err := database.DB()
    handle.Close()
    query := "bound search value"
    database.Where(clause.Like{Column: clause.Column{Name: "subject"}, Value: query})
    condition := "unused fragment"
    {
        condition := clause.Eq{Column: "id", Value: identityID}
        database.Where(condition)
    }
}
func anotherFixture() {
    condition := clause.Eq{Column: "id", Value: identityID}
    database.Where(condition)
}`)
	violations, err := inspectGORMContract("fixture.go", source)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("unexpected contract violations: %v", violations)
	}
}

func shouldSkipGORMContractDirectory(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "bin":
		return true
	default:
		return false
	}
}
