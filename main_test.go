package main

import (
	"hanamilsp/lsp"
	"os"
	"path/filepath"
	"testing"

	"github.com/matryer/is"
)

func TestGetSymbolNameToLookup(t *testing.T) {
	is := is.New(t)

	line := "           goal, key_results = transaction.call do"
	position := lsp.Position{
		Line:      91,
		Character: 31,
	}

	is.Equal(GetSymbolNameToLookup(line, position), "transaction")
}

func TestGetSymbolAndMethodNameToLookup(t *testing.T) {
	is := is.New(t)

	testCases := []struct {
		name           string
		line           string
		character      int
		expectedSym    string
		expectedMethod string
	}{
		{
			name:           "basic",
			line:           `goal_result = yield with_failure_prefix.call(prefix: "goal")`,
			character:      41,
			expectedSym:    "with_failure_prefix",
			expectedMethod: "call",
		},
		{
			name:           "basic",
			line:           `updated_visibility = yield apply_visibility.call(`,
			character:      42,
			expectedSym:    "apply_visibility",
			expectedMethod: "call",
		},
		{
			name:           "sym and method not found",
			line:           `"operations.with_failure_prefix"`,
			character:      18,
			expectedSym:    "",
			expectedMethod: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			position := lsp.Position{
				Line:      0,
				Character: tc.character,
			}

			sym, method := GetSymbolAndMethodNameToLookup(tc.line, position)
			is.Equal(sym, tc.expectedSym)
			is.Equal(method, tc.expectedMethod)
		})
	}
}

func TestGetIncludeDepsLine(t *testing.T) {
	is := is.New(t)

	b, err := os.ReadFile(filepath.Join("./testdata", "slices", "domain", "operations", "commands", "create_published_goal.rb"))
	if err != nil {
		t.Fatalf("could not read fixture file: %s", err)
	}
	document := string(b)

	testCases := []struct {
		symbolName   string
		expectedLine int
	}{
		{
			symbolName:   "transaction",
			expectedLine: 5,
		},
		{
			symbolName:   "create_key_result",
			expectedLine: 9,
		},
		{
			symbolName:   "get_collaboration", // doesn't exist
			expectedLine: -1,
		},
	}

	for _, tc := range testCases {
		receivedLine := GetIncludeDepsLine(document, tc.symbolName)
		is.Equal(receivedLine, tc.expectedLine)
	}
}

func TestHandleInitializeRequest(t *testing.T) {
	is := is.New(t)
	h := NewDefaultHandler()

	testuri := "testuri"
	req := lsp.InitializeRequest{
		Request: lsp.Request{
			RPC: "2.0",
			ID:  1,
		},
		Params: lsp.InitializeRequestParams{
			RootURI: lsp.DocumentURI(testuri),
		},
	}
	resp, err := h.handleInitializeRequest(req)

	t.Run("it does not return an error", func(t *testing.T) {
		is.NoErr(err)
	})

	t.Run("it returns the correct result", func(t *testing.T) {
		is.Equal(resp.Result, lsp.InitializeResult{
			Capabilities: lsp.ServerCapabilities{
				TextDocumentSync:   1,
				DefinitionProvider: true,
			},
			ServerInfo: lsp.ServerInfo{
				Name:    "hanamilsp",
				Version: "0",
			},
		})
	})

	t.Run("it sets the correct uri", func(t *testing.T) {
		is.Equal(string(h.State.RootURI), testuri)
	})
}

func TestHandleTextDocumentDefinition(t *testing.T) {
	is := is.New(t)

	rootURI := GetTestRootURI(t)
	currentURI := GetTestCurrentURI(t)
	testCases := []struct {
		Name             string
		CurrentURI       string
		CurrentPosition  lsp.Position
		ResponseLocation lsp.Location
		Err              error
	}{
		{
			Name:            "ErrorDocumentDoesNotExist",
			CurrentURI:      "baduri",
			CurrentPosition: lsp.Position{Line: 0, Character: 0},
			Err:             ErrorDocumentDoesNotExist{uri: "baduri"},
		},
		{
			Name:            "ErrorLineOutOfDocumentRange",
			CurrentURI:      currentURI,
			CurrentPosition: lsp.Position{Line: 9999, Character: 0},
			Err:             ErrorLineOutOfDocumentRange{uri: currentURI, line: 9999},
		},
		{
			Name:            "ErrorCouldNotParseSymbolAndMethodName",
			CurrentURI:      currentURI,
			CurrentPosition: lsp.Position{Line: 74, Character: 23},
			Err:             ErrorCouldNotParseSymbolAndMethodName{uri: currentURI, line: 74, rawLine: ""},
		},
		{
			Name:            "returns location of transaction file when called on the Deps line",
			CurrentURI:      currentURI,
			CurrentPosition: lsp.Position{Line: 5, Character: 13},
			ResponseLocation: lsp.Location{
				URI: rootURI + "/slices/domain/operations/transaction.rb",
				Range: lsp.Range{
					Start: lsp.Position{},
					End:   lsp.Position{},
				},
			},
		},
		{
			Name:            "returns location of validate_relationships file",
			CurrentURI:      currentURI,
			CurrentPosition: lsp.Position{Line: 79, Character: 18},
			ResponseLocation: lsp.Location{
				URI: rootURI + "/slices/domain/operations/commands/services/validate_relationships.rb",
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      13,
						Character: 14,
					},
					End: lsp.Position{
						Line:      13,
						Character: 14,
					},
				},
			},
		},
		{
			Name:            "returns location of the call method in the validate_relationships file",
			CurrentURI:      currentURI,
			CurrentPosition: lsp.Position{Line: 79, Character: 41},
			ResponseLocation: lsp.Location{
				URI: rootURI + "/slices/domain/operations/commands/services/validate_relationships.rb",
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      13,
						Character: 14,
					},
					End: lsp.Position{
						Line:      13,
						Character: 14,
					},
				},
			},
		},
		{
			Name:            "returns location of an aliased Dep correctly (apply_visibility)",
			CurrentURI:      currentURI,
			CurrentPosition: lsp.Position{Line: 123, Character: 41},
			ResponseLocation: lsp.Location{
				URI: rootURI + "/slices/domain/operations/commands/services/visibility/apply_and_transform.rb",
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      31,
						Character: 16,
					},
					End: lsp.Position{
						Line:      31,
						Character: 16,
					},
				},
			},
		},
		{
			Name:            "returns location of metadata_repo correctly, when there's multiple method calls on same line",
			CurrentURI:      currentURI,
			CurrentPosition: lsp.Position{Line: 104, Character: 30},
			ResponseLocation: lsp.Location{
				URI: rootURI + "/slices/domain/repositories/goal_metadata_repo.rb",
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      3,
						Character: 10,
					},
					End: lsp.Position{
						Line:      3,
						Character: 10,
					},
				},
			},
		},
		{
			Name:            "returns location of get_collaborations correctly, which is in a different slice",
			CurrentURI:      currentURI,
			CurrentPosition: lsp.Position{Line: 189, Character: 18},
			ResponseLocation: lsp.Location{
				URI: rootURI + "/slices/collaborations/operations/queries/get_collaboration.rb",
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      13,
						Character: 12,
					},
					End: lsp.Position{
						Line:      13,
						Character: 12,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			h := NewTestHandler(t)
			req := NewTestDefinitionRequest(
				withCurrentURI(tc.CurrentURI),
				withCurrentPosition(tc.CurrentPosition),
			)

			resp, err := h.handleTextDocumentDefinition(req)
			if err != nil {
				is.Equal(err, tc.Err)
			} else {
				is.Equal(resp.Result, tc.ResponseLocation)
			}
		})
	}
}

func NewTestHandler(t *testing.T) *Handler {
	h := NewDefaultHandler()

	fixture, err := os.ReadFile(filepath.Join("./testdata", "slices", "domain", "operations", "commands", "create_published_goal.rb"))
	if err != nil {
		t.Fatalf("could not read fixture file: %s", err)
	}

	rootURI := GetTestRootURI(t)
	currentURI := GetTestCurrentURI(t)

	h.State.RootURI = lsp.DocumentURI(rootURI)
	h.State.Documents[currentURI] = string(fixture)

	return h
}

func GetTestRootURI(t *testing.T) string {
	rootURI, err := filepath.Abs(filepath.Join("testdata"))
	if err != nil {
		t.Fatalf("could not init rootURI")
	}

	return rootURI
}

func GetTestCurrentURI(t *testing.T) string {
	return string(GetTestRootURI(t)) + "/slices/domain/operations/create_published_goal.rb"
}

type OptFunc func(*Opts)

type Opts struct {
	CurrentURI      string
	CurrentPosition lsp.Position
}

func defaultOpts() Opts {
	return Opts{
		CurrentURI: "",
		CurrentPosition: lsp.Position{
			Line:      0,
			Character: 0,
		},
	}
}

func withCurrentURI(uri string) OptFunc {
	return func(o *Opts) {
		o.CurrentURI = uri
	}
}

func withCurrentPosition(pos lsp.Position) OptFunc {
	return func(o *Opts) {
		o.CurrentPosition = pos
	}
}

func NewTestDefinitionRequest(opts ...OptFunc) lsp.DefinitionRequest {
	o := defaultOpts()
	for _, fn := range opts {
		fn(&o)
	}
	return lsp.DefinitionRequest{
		Request: lsp.Request{
			RPC: "2.0",
			ID:  1,
		},
		Params: lsp.DefinitionParams{
			TextDocumentPositionParams: lsp.TextDocumentPositionParams{
				TextDocument: lsp.TextDocumentIdentifier{
					URI: o.CurrentURI,
				},
				Position: o.CurrentPosition,
			},
		},
	}
}
