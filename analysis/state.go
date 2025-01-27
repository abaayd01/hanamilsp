package analysis

import (
	"hanamilsp/lsp"
	"log"
	"strings"
)

type State struct {
	// Map of file names to contents
	Documents map[string]string
	RootURI   lsp.DocumentURI
	Logger    *log.Logger
}

func NewState(
	logger *log.Logger,
) *State {
	return &State{
		Documents: map[string]string{},
		Logger:    logger,
	}
}

func getDiagnosticsForFile(text string) []lsp.Diagnostic {
	diagnostics := []lsp.Diagnostic{}
	for row, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "VS Code") {
			idx := strings.Index(line, "VS Code")
			diagnostics = append(diagnostics, lsp.Diagnostic{
				Range:    LineRange(row, idx, idx+len("VS Code")),
				Severity: 1,
				Source:   "Common Sense",
				Message:  "Please make sure we use good language in this video",
			})
		}

		if strings.Contains(line, "Neovim") {
			idx := strings.Index(line, "Neovim")
			diagnostics = append(diagnostics, lsp.Diagnostic{
				Range:    LineRange(row, idx, idx+len("Neovim")),
				Severity: 2,
				Source:   "Common Sense",
				Message:  "Great choice :)",
			})

		}
	}

	return diagnostics
}

func (s *State) OpenDocument(uri, text string) []lsp.Diagnostic {
	s.Documents[uri] = text

	return getDiagnosticsForFile(text)
}

func (s *State) UpdateDocument(uri, text string) []lsp.Diagnostic {
	s.Documents[uri] = text

	return getDiagnosticsForFile(text)
}

// func (s *State) TextDocumentCodeAction(id int, uri string) lsp.TextDocumentCodeActionResponse {
// 	text := s.Documents[uri]
//
// 	actions := []lsp.CodeAction{}
// 	for row, line := range strings.Split(text, "\n") {
// 		idx := strings.Index(line, "VS Code")
// 		if idx >= 0 {
// 			replaceChange := map[string][]lsp.TextEdit{}
// 			replaceChange[uri] = []lsp.TextEdit{
// 				{
// 					Range:   LineRange(row, idx, idx+len("VS Code")),
// 					NewText: "Neovim",
// 				},
// 			}
//
// 			actions = append(actions, lsp.CodeAction{
// 				Title: "Replace VS C*de with a superior editor",
// 				Edit:  &lsp.WorkspaceEdit{Changes: replaceChange},
// 			})
//
// 			censorChange := map[string][]lsp.TextEdit{}
// 			censorChange[uri] = []lsp.TextEdit{
// 				{
// 					Range:   LineRange(row, idx, idx+len("VS Code")),
// 					NewText: "VS C*de",
// 				},
// 			}
//
// 			actions = append(actions, lsp.CodeAction{
// 				Title: "Censor to VS C*de",
// 				Edit:  &lsp.WorkspaceEdit{Changes: censorChange},
// 			})
// 		}
// 	}
//
// 	response := lsp.TextDocumentCodeActionResponse{
// 		Response: lsp.Response{
// 			RPC: "2.0",
// 			ID:  &id,
// 		},
// 		Result: actions,
// 	}
//
// 	return response
// }

// func (s *State) TextDocumentCompletion(id int, uri string) lsp.CompletionResponse {
//
// 	// Ask your static analysis tools to figure out good completions
// 	items := []lsp.CompletionItem{
// 		{
// 			Label:         "Neovim (BTW)",
// 			Detail:        "Very cool editor",
// 			Documentation: "Fun to watch in videos. Don't forget to like & subscribe to streamers using it :)",
// 		},
// 	}
//
// 	response := lsp.CompletionResponse{
// 		Response: lsp.Response{
// 			RPC: "2.0",
// 			ID:  &id,
// 		},
// 		Result: items,
// 	}
//
// 	return response
// }

func LineRange(line, start, end int) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{
			Line:      line,
			Character: start,
		},
		End: lsp.Position{
			Line:      line,
			Character: end,
		},
	}
}
