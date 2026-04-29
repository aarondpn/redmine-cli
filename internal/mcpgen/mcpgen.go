package mcpgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const (
	opsDir            = "internal/ops"
	generatedToolsOut = "internal/mcpserver/zz_generated_tools.go"
	generatedDocsOut  = "docs/mcp/tools.md"
)

type Outputs struct {
	ToolsGo []byte
	DocsMD  []byte
}

type ToolSpec struct {
	FuncName    string
	Name        string
	Description string
	Category    string
	Writes      bool
	Handler     string
	InputType   string
	OutputType  string
	InputDoc    *StructDoc
}

type StructDoc struct {
	Name   string
	Fields []FieldDoc
}

type FieldDoc struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

func Write(repoRoot string) error {
	out, err := Generate(repoRoot)
	if err != nil {
		return err
	}

	toolsPath := filepath.Join(repoRoot, generatedToolsOut)
	if err := os.WriteFile(toolsPath, out.ToolsGo, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", toolsPath, err)
	}

	docsPath := filepath.Join(repoRoot, generatedDocsOut)
	if err := os.MkdirAll(filepath.Dir(docsPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(docsPath), err)
	}
	if err := os.WriteFile(docsPath, out.DocsMD, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", docsPath, err)
	}

	return nil
}

func Generate(repoRoot string) (Outputs, error) {
	specs, err := parseSpecs(filepath.Join(repoRoot, opsDir))
	if err != nil {
		return Outputs{}, err
	}

	toolsGo, err := renderTools(specs)
	if err != nil {
		return Outputs{}, err
	}

	docsMD, err := renderDocs(specs)
	if err != nil {
		return Outputs{}, err
	}

	return Outputs{ToolsGo: toolsGo, DocsMD: docsMD}, nil
}

func parseSpecs(dir string) ([]ToolSpec, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read ops package dir: %w", err)
	}

	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if file.Name.Name != "ops" {
			continue
		}
		files[path] = file
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("ops package not found in %s", dir)
	}

	typeDocs := map[string]*StructDoc{}
	fileNames := make([]string, 0, len(files))
	for name := range files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	for _, name := range fileNames {
		file := files[name]
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts := spec.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				typeDocs[ts.Name.Name] = buildStructDoc(fset, ts.Name.Name, st)
			}
		}
	}

	var out []ToolSpec
	for _, name := range fileNames {
		file := files[name]
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Doc == nil {
				continue
			}
			meta := parseDirectives(fd.Doc.List)
			if meta.Name == "" {
				continue
			}
			spec, err := buildSpec(fset, fd, meta, typeDocs)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", fd.Name.Name, err)
			}
			out = append(out, spec)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})

	return out, nil
}

type directiveMeta struct {
	Name        string
	Description string
	Category    string
	Writes      bool
	Handler     string
}

func parseDirectives(lines []*ast.Comment) directiveMeta {
	var meta directiveMeta
	for _, line := range lines {
		text := strings.TrimSpace(strings.TrimPrefix(line.Text, "//"))
		switch {
		case strings.HasPrefix(text, "mcpgen:tool "):
			meta.Name = strings.TrimSpace(strings.TrimPrefix(text, "mcpgen:tool "))
		case strings.HasPrefix(text, "mcpgen:description "):
			meta.Description = strings.TrimSpace(strings.TrimPrefix(text, "mcpgen:description "))
		case strings.HasPrefix(text, "mcpgen:category "):
			meta.Category = strings.TrimSpace(strings.TrimPrefix(text, "mcpgen:category "))
		case text == "mcpgen:writes":
			meta.Writes = true
		case strings.HasPrefix(text, "mcpgen:handler "):
			meta.Handler = strings.TrimSpace(strings.TrimPrefix(text, "mcpgen:handler "))
		}
	}
	return meta
}

func buildSpec(fset *token.FileSet, fd *ast.FuncDecl, meta directiveMeta, typeDocs map[string]*StructDoc) (ToolSpec, error) {
	if meta.Description == "" {
		return ToolSpec{}, fmt.Errorf("missing mcpgen:description")
	}
	if meta.Category == "" {
		return ToolSpec{}, fmt.Errorf("missing mcpgen:category")
	}
	if fd.Type.Params == nil || len(fd.Type.Params.List) != 3 {
		return ToolSpec{}, fmt.Errorf("expected func(ctx, client, input) signature")
	}
	if fd.Type.Results == nil || len(fd.Type.Results.List) != 2 {
		return ToolSpec{}, fmt.Errorf("expected two return values")
	}

	inExpr := fd.Type.Params.List[2].Type
	outExpr := fd.Type.Results.List[0].Type
	inputDoc := inputDocForType(fset, inExpr, typeDocs)

	return ToolSpec{
		FuncName:    fd.Name.Name,
		Name:        meta.Name,
		Description: meta.Description,
		Category:    meta.Category,
		Writes:      meta.Writes,
		Handler:     meta.Handler,
		InputType:   qualifyExpr(inExpr),
		OutputType:  qualifyExpr(outExpr),
		InputDoc:    inputDoc,
	}, nil
}

func qualifyExpr(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return "ops." + v.Name
	case *ast.SelectorExpr:
		return exprString(token.NewFileSet(), v)
	case *ast.StarExpr:
		return "*" + qualifyExpr(v.X)
	case *ast.StructType:
		return "struct{}"
	case *ast.ArrayType:
		return "[]" + qualifyExpr(v.Elt)
	default:
		return exprString(token.NewFileSet(), expr)
	}
}

func inputDocForType(fset *token.FileSet, expr ast.Expr, typeDocs map[string]*StructDoc) *StructDoc {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	doc, ok := typeDocs[ident.Name]
	if !ok {
		return nil
	}
	copyDoc := *doc
	copyDoc.Fields = append([]FieldDoc(nil), doc.Fields...)
	for i := range copyDoc.Fields {
		copyDoc.Fields[i] = doc.Fields[i]
	}
	_ = fset
	return &copyDoc
}

func buildStructDoc(fset *token.FileSet, name string, st *ast.StructType) *StructDoc {
	doc := &StructDoc{Name: name}
	if st.Fields == nil {
		return doc
	}
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 || field.Tag == nil {
			continue
		}
		tagValue, err := strconvUnquote(field.Tag.Value)
		if err != nil {
			continue
		}
		tag := reflect.StructTag(tagValue)
		jsonName, required := parseJSONTag(tag.Get("json"))
		if jsonName == "" || jsonName == "-" {
			continue
		}
		doc.Fields = append(doc.Fields, FieldDoc{
			Name:        jsonName,
			Type:        exprString(fset, field.Type),
			Required:    required,
			Description: tag.Get("jsonschema"),
		})
	}
	return doc
}

func parseJSONTag(tag string) (string, bool) {
	if tag == "" {
		return "", false
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	required := true
	for _, part := range parts[1:] {
		if part == "omitempty" {
			required = false
		}
	}
	return name, required
}

func renderTools(specs []ToolSpec) ([]byte, error) {
	needsModels := false
	for _, spec := range specs {
		if strings.Contains(spec.InputType, "models.") || strings.Contains(spec.OutputType, "models.") {
			needsModels = true
			break
		}
	}

	var buf bytes.Buffer
	buf.WriteString("// Code generated by go generate ./...; DO NOT EDIT.\n")
	buf.WriteString("package mcpserver\n\n")
	buf.WriteString("import (\n")
	buf.WriteString("\t\"github.com/modelcontextprotocol/go-sdk/mcp\"\n\n")
	buf.WriteString("\t\"github.com/aarondpn/redmine-cli/v2/internal/api\"\n")
	if needsModels {
		buf.WriteString("\t\"github.com/aarondpn/redmine-cli/v2/internal/models\"\n")
	}
	buf.WriteString("\t\"github.com/aarondpn/redmine-cli/v2/internal/ops\"\n")
	buf.WriteString(")\n\n")
	buf.WriteString("func registerGeneratedTools(s *mcp.Server, client *api.Client, opts Options) {\n")
	for _, spec := range specs {
		if spec.Handler != "" {
			fmt.Fprintf(&buf, "\t%s(s, client, opts)\n", spec.Handler)
			continue
		}
		fmt.Fprintf(&buf, "\tregisterToolSpec(s, client, opts, toolSpec[%s, %s]{\n", spec.InputType, spec.OutputType)
		fmt.Fprintf(&buf, "\t\tName:        %q,\n", spec.Name)
		fmt.Fprintf(&buf, "\t\tDescription: %q,\n", spec.Description)
		if spec.Writes {
			buf.WriteString("\t\tWrites:      true,\n")
		}
		fmt.Fprintf(&buf, "\t\tCall:        ops.%s,\n", spec.FuncName)
		buf.WriteString("\t})\n")
	}
	buf.WriteString("}\n")

	return format.Source(buf.Bytes())
}

func renderDocs(specs []ToolSpec) ([]byte, error) {
	grouped := map[string][]ToolSpec{}
	var categories []string
	for _, spec := range specs {
		if _, ok := grouped[spec.Category]; !ok {
			categories = append(categories, spec.Category)
		}
		grouped[spec.Category] = append(grouped[spec.Category], spec)
	}
	sort.Strings(categories)
	for _, category := range categories {
		sort.Slice(grouped[category], func(i, j int) bool {
			return grouped[category][i].Name < grouped[category][j].Name
		})
	}

	var buf bytes.Buffer
	buf.WriteString("# MCP Tools\n\n")
	buf.WriteString("Generated from annotated ops functions. Do not edit by hand.\n\n")
	for _, category := range categories {
		buf.WriteString("## " + category + "\n\n")
		for _, spec := range grouped[category] {
			mode := "read"
			if spec.Writes {
				mode = "write"
			}
			buf.WriteString("### `" + spec.Name + "`\n\n")
			buf.WriteString(spec.Description + "\n\n")
			buf.WriteString("- Mode: `" + mode + "`\n")
			buf.WriteString("- Source: `ops." + spec.FuncName + "`\n\n")
			if spec.InputDoc == nil || len(spec.InputDoc.Fields) == 0 {
				buf.WriteString("Parameters: none.\n\n")
				continue
			}
			buf.WriteString("| Parameter | Type | Required | Description |\n")
			buf.WriteString("| --- | --- | --- | --- |\n")
			for _, field := range spec.InputDoc.Fields {
				required := "no"
				if field.Required {
					required = "yes"
				}
				fmt.Fprintf(&buf, "| `%s` | `%s` | %s | %s |\n", field.Name, field.Type, required, escapePipes(field.Description))
			}
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), nil
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, expr)
	return buf.String()
}

func escapePipes(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func strconvUnquote(s string) (string, error) {
	if len(s) < 2 {
		return "", fmt.Errorf("short quoted string")
	}
	return s[1 : len(s)-1], nil
}
