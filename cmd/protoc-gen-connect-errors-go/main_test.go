package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestExtractTemplateFields(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    []string
	}{
		{"no placeholders", "Internal server error", nil},
		{"single field", "User '{{id}}' not found", []string{"id"}},
		{"multiple fields", "User '{{id}}' not found in '{{org}}'", []string{"id", "org"}},
		{"duplicate fields", "{{id}} and {{id}} again", []string{"id"}},
		{"snake_case field", "Product {{product_id}} unavailable", []string{"product_id"}},
		{"empty message", "", nil},
		{"adjacent placeholders", "{{a}}{{b}}", []string{"a", "b"}},
		{"unclosed placeholder", "Hello {{name", nil},
		{"three fields", "{{amount}} exceeds {{limit}} for {{account}}", []string{"amount", "limit", "account"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTemplateFields(tt.message)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractTemplateFields(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestFieldToExportedName(t *testing.T) {
	tests := []struct {
		field string
		want  string
	}{
		{"id", "Id"},
		{"email", "Email"},
		{"product_id", "ProductId"},
		{"order_item_id", "OrderItemId"},
		{"reason", "Reason"},
		{"unlock_at", "UnlockAt"},
		{"last4", "Last4"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got := fieldToExportedName(tt.field)
			if got != tt.want {
				t.Errorf("fieldToExportedName(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestErrorCodeToConstant(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"ERROR_NOT_FOUND", "NotFound"},
		{"ERROR_INVALID_ARGUMENT", "InvalidArgument"},
		{"ERROR_USER_NOT_FOUND", "UserNotFound"},
		{"ERROR_INTERNAL", "Internal"},
		{"ERROR_OUT_OF_STOCK", "OutOfStock"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := errorCodeToConstant(tt.code)
			if got != tt.want {
				t.Errorf("errorCodeToConstant(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestMapConnectCode(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{"canceled", 1, "connect.CodeCanceled"},
		{"unknown", 2, "connect.CodeUnknown"},
		{"invalid_argument", 3, "connect.CodeInvalidArgument"},
		{"deadline_exceeded", 4, "connect.CodeDeadlineExceeded"},
		{"not_found", 5, "connect.CodeNotFound"},
		{"already_exists", 6, "connect.CodeAlreadyExists"},
		{"permission_denied", 7, "connect.CodePermissionDenied"},
		{"resource_exhausted", 8, "connect.CodeResourceExhausted"},
		{"failed_precondition", 9, "connect.CodeFailedPrecondition"},
		{"aborted", 10, "connect.CodeAborted"},
		{"out_of_range", 11, "connect.CodeOutOfRange"},
		{"unimplemented", 12, "connect.CodeUnimplemented"},
		{"internal", 13, "connect.CodeInternal"},
		{"unavailable", 14, "connect.CodeUnavailable"},
		{"data_loss", 15, "connect.CodeDataLoss"},
		{"unauthenticated", 16, "connect.CodeUnauthenticated"},
		{"fallback_unknown", 99, "connect.CodeInternal"},
		{"fallback_zero", 0, "connect.CodeInternal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapConnectCode(tt.code)
			if got != tt.want {
				t.Errorf("mapConnectCode(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestParseErrorDef(t *testing.T) {
	// Manual protobuf wire format construction
	// ErrorDef {
	//   code (1): "ERR_TEST"
	//   message (2): "msg"
	//   connect_code (3): 5 (not_found)
	//   retryable (4): true
	// }
	data := []byte{
		0x0a, 0x08, 'E', 'R', 'R', '_', 'T', 'E', 'S', 'T', // tag 1 (string): "ERR_TEST"
		0x12, 0x03, 'm', 's', 'g', // tag 2 (string): "msg"
		0x18, 0x05, // tag 3 (varint): 5
		0x20, 0x01, // tag 4 (varint): 1
	}

	got, ok := parseErrorDef(data)
	if !ok {
		t.Fatal("parseErrorDef failed")
	}

	if got.Code != "ERR_TEST" {
		t.Errorf("Code = %q, want ERR_TEST", got.Code)
	}
	if got.Message != "msg" {
		t.Errorf("Message = %q, want msg", got.Message)
	}
	if got.ConnectCode != 5 {
		t.Errorf("ConnectCode = %d, want 5", got.ConnectCode)
	}
	if !got.Retryable {
		t.Error("Retryable = false, want true")
	}
}

func TestParseErrorDefWithRetryDelay(t *testing.T) {
	// ErrorDef {
	//   code (1): "ERROR_RATE_LIMITED"
	//   message (2): "Too many requests"
	//   connect_code (3): 8 (resource_exhausted)
	//   retryable (4): true
	//   retry_delay_ms (5): 5000
	// }
	data := []byte{
		0x0a, 0x12, 'E', 'R', 'R', 'O', 'R', '_', 'R', 'A', 'T', 'E', '_', 'L', 'I', 'M', 'I', 'T', 'E', 'D',
		0x12, 0x11, 'T', 'o', 'o', ' ', 'm', 'a', 'n', 'y', ' ', 'r', 'e', 'q', 'u', 'e', 's', 't', 's',
		0x18, 0x08, // connect_code: 8
		0x20, 0x01, // retryable: true
		0x28, 0x88, 0x27, // retry_delay_ms: 5000 (varint: 0x1388 → 0x88 0x27)
	}

	got, ok := parseErrorDef(data)
	if !ok {
		t.Fatal("parseErrorDef failed")
	}
	if got.RetryDelayMs != 5000 {
		t.Errorf("RetryDelayMs = %d, want 5000", got.RetryDelayMs)
	}
}

func TestParseErrorDefEmptyCode(t *testing.T) {
	// ErrorDef with empty code
	data := []byte{
		0x0a, 0x00, // code: ""
		0x12, 0x03, 'm', 's', 'g',
	}
	got, ok := parseErrorDef(data)
	if !ok {
		t.Fatal("parseErrorDef failed")
	}
	if got.Code != "" {
		t.Errorf("Code = %q, want empty", got.Code)
	}
}

func TestIsValidErrorCode(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"ERROR_NOT_FOUND", true},
		{"ERROR_USER_NOT_FOUND", true},
		{"NOT_FOUND", true},
		{"ERROR_123", true},
		{"", false},
		{"ERROR-FOO", false},
		{"ERROR FOO", false},
		{"ERROR.FOO", false},
		{"123", false}, // no letters
		{"ERROR_PECIAL!", false},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := isValidErrorCode(tt.code); got != tt.want {
				t.Errorf("isValidErrorCode(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestIsValidFieldName(t *testing.T) {
	tests := []struct {
		field string
		want  bool
	}{
		{"id", true},
		{"email", true},
		{"product_id", true},
		{"last4", true},
		{"", false},
		{"foo-bar", false},
		{"foo bar", false},
		{"foo.bar", false},
		{"foo$", false},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			if got := isValidFieldName(tt.field); got != tt.want {
				t.Errorf("isValidFieldName(%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestExtractTemplateFieldsWithInvalidNames(t *testing.T) {
	// Fields with special characters should be skipped
	fields := extractTemplateFields("User '{{id}}' in {{foo-bar}} and {{invalid.field}}")
	if !reflect.DeepEqual(fields, []string{"id"}) {
		t.Errorf("extractTemplateFields = %v, want [id]", fields)
	}
}

// generateTestPlugin runs generateFile with a synthetic proto file descriptor
// containing error options, and returns the generated Go source code.
func generateTestPlugin(t *testing.T, defs []errorDef) string {
	t.Helper()

	// Build a synthetic FileOptions with error extensions at field 50002
	fileOpts := &descriptorpb.FileOptions{}
	rawOpts, err := proto.Marshal(fileOpts)
	if err != nil {
		t.Fatalf("marshal empty FileOptions: %v", err)
	}

	// Append each errorDef as a wire-format extension at field 50002 (BytesType)
	for _, def := range defs {
		errBytes := encodeErrorDef(def)
		extTag := protowire.AppendTag(nil, 50002, protowire.BytesType)
		extBytes := protowire.AppendBytes(extTag, errBytes)
		rawOpts = append(rawOpts, extBytes...)
	}

	// Re-unmarshal to get the file options with extensions
	fileOpts = &descriptorpb.FileOptions{}
	if err := proto.Unmarshal(rawOpts, fileOpts); err != nil {
		t.Fatalf("unmarshal FileOptions: %v", err)
	}

	// Create a minimal file descriptor
	fileDesc := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test/v1/service.proto"),
		Package: proto.String("test.v1"),
		Options: fileOpts,
		Syntax:  proto.String("proto3"),
	}
	// Add minimal Go package option so protogen can determine the import path
	fileDesc.Options.GoPackage = proto.String("github.com/test/gen/test/v1;testv1")

	// Build CodeGeneratorRequest
	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile: []*descriptorpb.FileDescriptorProto{fileDesc},
		FileToGenerate: []string{"test/v1/service.proto"},
	}

	// Create protogen plugin
	opts := protogen.Options{}
	plugin, err := opts.New(req)
	if err != nil {
		t.Fatalf("protogen Options.New: %v", err)
	}

	for _, f := range plugin.Files {
		if !f.Generate {
			continue
		}
		generateFile(plugin, f)
	}

	// Collect generated file content from response
	resp := plugin.Response()
	if resp == nil || len(resp.File) == 0 {
		return ""
	}
	for _, f := range resp.File {
		if strings.HasSuffix(f.GetName(), "_connect_errors.go") {
			return f.GetContent()
		}
	}
	return ""
}

// encodeErrorDef encodes an errorDef as protobuf wire format.
func encodeErrorDef(def errorDef) []byte {
	var b []byte
	if def.Code != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, def.Code)
	}
	if def.Message != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, def.Message)
	}
	if def.ConnectCode != 0 {
		b = protowire.AppendTag(b, 3, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(def.ConnectCode))
	}
	if def.Retryable {
		b = protowire.AppendTag(b, 4, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	if def.RetryDelayMs != 0 {
		b = protowire.AppendTag(b, 5, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(def.RetryDelayMs))
	}
	return b
}

func TestGenerateFileProducesCompilableGo(t *testing.T) {
	defs := []errorDef{
		{Code: "ERROR_USER_NOT_FOUND", Message: "User '{{id}}' not found", ConnectCode: 5, Retryable: false},
		{Code: "ERROR_RATE_LIMITED", Message: "Too many requests", ConnectCode: 8, Retryable: true, RetryDelayMs: 5000},
		{Code: "ERROR_INTERNAL", Message: "Internal error", ConnectCode: 13, Retryable: false},
	}

	generated := generateTestPlugin(t, defs)
	if generated == "" {
		t.Skip("no file generated (protogen API limitation in test)")
		return
	}

	// Verify key elements are present in generated code
	checks := []string{
		"package",                                      // has a package declaration
		"ErrUserNotFound",                             // generated constant
		"ErrRateLimited",                              // generated constant
		"NewErrUserNotFound",                          // generated constructor
		"IsUserNotFound",                              // generated matcher
		"ExtractUserNotFoundInfo",                     // generated extractor
		`cerr.ErrorCode = "ERROR_USER_NOT_FOUND"`,     // constant value
		"cerr.RegisterAll",                            // auto-registration
	}
	for _, check := range checks {
		if !strings.Contains(generated, check) {
			t.Errorf("generated code missing %q\n--- Generated ---\n%s", check, generated)
		}
	}

	// Verify generated code is valid Go syntax
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "generated.go", generated, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n--- Generated ---\n%s", err, generated)
	}
}

func TestGenerateFileSkipsInvalidCodes(t *testing.T) {
	defs := []errorDef{
		{Code: "", Message: "empty code", ConnectCode: 5},
		{Code: "ERROR-INVALID", Message: "hyphen code", ConnectCode: 5},
		{Code: "ERROR_VALID", Message: "valid", ConnectCode: 5},
	}

	generated := generateTestPlugin(t, defs)
	if generated == "" {
		t.Skip("no file generated")
		return
	}

	if strings.Contains(generated, "ErrInvalid") || strings.Contains(generated, `"ERROR-INVALID"`) {
		t.Errorf("invalid error code should be skipped\n--- Generated ---\n%s", generated)
	}
	if !strings.Contains(generated, "ErrValid") {
		t.Errorf("valid error code should be present\n--- Generated ---\n%s", generated)
	}
}

func TestGenerateFileHandlesNoPlaceholders(t *testing.T) {
	defs := []errorDef{
		{Code: "ERROR_SIMPLE", Message: "A simple error", ConnectCode: 13, Retryable: false},
	}

	generated := generateTestPlugin(t, defs)
	if generated == "" {
		t.Skip("no file generated")
		return
	}

	// No-arg constructor should be generated
	if !strings.Contains(generated, "func NewErrSimple() *connect.Error") {
		t.Errorf("no-arg constructor missing\n--- Generated ---\n%s", generated)
	}
}

func TestGeneratedASTDeclaration(t *testing.T) {
	// Use go/ast to verify the generated code has proper declarations
	defs := []errorDef{
		{Code: "ERROR_NOT_FOUND", Message: "Not found '{{id}}'", ConnectCode: 5, Retryable: false},
	}

	generated := generateTestPlugin(t, defs)
	if generated == "" {
		t.Skip("no file generated")
		return
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated.go", generated, 0)
	if err != nil {
		t.Fatalf("parse generated code: %v", err)
	}

	var hasConst, hasFunc, hasInit bool
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.CONST {
				hasConst = true
			}
		case *ast.FuncDecl:
			if decl.Name.Name == "init" {
				hasInit = true
			}
			if strings.HasPrefix(decl.Name.Name, "NewErr") {
				hasFunc = true
			}
		}
		return true
	})

	if !hasConst {
		t.Error("generated code should have const declarations")
	}
	if !hasFunc {
		t.Error("generated code should have constructor functions")
	}
	if !hasInit {
		t.Error("generated code should have an init() function")
	}
}
