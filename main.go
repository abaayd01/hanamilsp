package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hanamilsp/analysis"
	"hanamilsp/lsp"
	"hanamilsp/rpc"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"
)

func main() {
	logger := getLogger("out.log")
	logger.Println("Started hanamilsp...")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(rpc.Split)

	state := analysis.NewState(
		logger,
	)
	writer := os.Stdout

	handler := NewHandler(logger, writer, state)

	for scanner.Scan() {
		msg := scanner.Bytes()
		method, contents, err := rpc.DecodeMessage(msg)
		if err != nil {
			logger.Printf("Got an error: %s", err)
			continue
		}

		handler.handleMessage(method, contents)
	}
}

type MsgMethod string

const (
	INITIALIZE                        = "initialize"
	TEXT_DOCUMENT_DID_OPEN            = "textDocument/didOpen"
	TEXT_DOCUMENT_DID_CHANGE          = "textDocument/didChange"
	TEXT_DOCUMENT_DEFINITION          = "textDocument/definition"
	TEXT_DOCUMENT_PUBLISH_DIAGNOSTICS = "textDocument/publishDiagnostics"
)

type Handler struct {
	Logger *log.Logger
	Writer io.Writer
	State  *analysis.State
}

func NewHandler(
	logger *log.Logger,
	writer io.Writer,
	state *analysis.State,
) *Handler {
	return &Handler{
		Logger: logger,
		Writer: writer,
		State:  state,
	}
}

func NewDefaultHandler() *Handler {
	logger := getLogger("out.log")
	state := analysis.NewState(
		logger,
	)
	writer := os.Stdout
	handler := NewHandler(logger, writer, state)
	return handler
}

func (h *Handler) handleMessage(method string, contents []byte) {
	h.Logger.Printf("received msg with method: %s", method)

	switch method {
	case INITIALIZE:
		handle(h, method, contents, h.handleInitializeRequest)
	case TEXT_DOCUMENT_DID_OPEN:
		handle(h, method, contents, h.handleTextDocumentDidOpen)
	case TEXT_DOCUMENT_DID_CHANGE:
		handleSlice(h, method, contents, h.handleTextDocumentDidChange)
	case TEXT_DOCUMENT_DEFINITION:
		handle(h, method, contents, h.handleTextDocumentDefinition)
	}
}

func handle[T any, R ResponseConstraint](
	h *Handler,
	method string,
	contents []byte,
	handlerFunc func(T) (R, error),
) {
	var v T
	if err := json.Unmarshal(contents, &v); err != nil {
		h.Logger.Printf("error: unable to unmarshal message for method '%s', err: %s", method, err)
		return
	}

	msg, err := handlerFunc(v)
	if err != nil {
		h.Logger.Printf("error: got err back from handlerFunc for method: %s", method)
		h.Logger.Printf("error: %s", err)
		return
	}

	writeResponse(h.Writer, msg)
}

type SliceConstraint[T any] interface {
	~[]T
}

type ResponseConstraint interface {
	ResponseMarker()
}

func handleSlice[T any, R ResponseConstraint, K SliceConstraint[R]](
	h *Handler,
	method string,
	contents []byte,
	handlerFunc func(*T) K,
) {
	var v T
	if err := json.Unmarshal(contents, &v); err != nil {
		h.Logger.Printf("error: unable to unmarshal message for method '%s', err: %s", method, err)
		return
	}

	msg := handlerFunc(&v)
	if msg == nil {
		h.Logger.Printf("error: got nil back from handlerFunc for method: %s", method)
		return
	}

	for _, r := range msg {
		writeResponse(h.Writer, r)
	}
}

func (h *Handler) handleInitializeRequest(request lsp.InitializeRequest) (lsp.InitializeResponse, error) {
	h.State.RootURI = request.Params.RootURI
	msg := lsp.NewInitializeResponse(&request.ID)
	return msg, nil
}

func (h *Handler) handleTextDocumentDidOpen(request lsp.DidOpenTextDocumentNotification) (lsp.PublishDiagnosticsNotification, error) {
	h.Logger.Printf("Opened: %s", request.Params.TextDocument.URI)
	diagnostics := h.State.OpenDocument(request.Params.TextDocument.URI, request.Params.TextDocument.Text)
	msg := lsp.PublishDiagnosticsNotification{
		Notification: lsp.Notification{
			RPC:    "2.0",
			Method: TEXT_DOCUMENT_PUBLISH_DIAGNOSTICS,
		},
		Params: lsp.PublishDiagnosticsParams{
			URI:         request.Params.TextDocument.URI,
			Diagnostics: diagnostics,
		},
	}
	return msg, nil
}

func (h *Handler) handleTextDocumentDidChange(request *lsp.TextDocumentDidChangeNotification) []lsp.PublishDiagnosticsNotification {
	h.Logger.Printf("Changed: %s", request.Params.TextDocument.URI)
	var notifications []lsp.PublishDiagnosticsNotification
	for _, change := range request.Params.ContentChanges {
		diagnostics := h.State.UpdateDocument(request.Params.TextDocument.URI, change.Text)
		notifications = append(notifications, lsp.PublishDiagnosticsNotification{
			Notification: lsp.Notification{
				RPC:    "2.0",
				Method: TEXT_DOCUMENT_PUBLISH_DIAGNOSTICS,
			},
			Params: lsp.PublishDiagnosticsParams{
				URI:         request.Params.TextDocument.URI,
				Diagnostics: diagnostics,
			},
		})
	}

	return notifications
}

type ErrorDocumentDoesNotExist struct {
	uri string
}

func (e ErrorDocumentDoesNotExist) Error() string {
	return fmt.Sprintf("document does not exist in state, uri: %s", e.uri)
}

type ErrorLineOutOfDocumentRange struct {
	uri  string
	line int
}

func (e ErrorLineOutOfDocumentRange) Error() string {
	return fmt.Sprintf("line '%d' does not exist in document with uri: %s", e.line, e.uri)
}

type ErrorCouldNotParseSymbolAndMethodName struct {
	uri     string
	line    int
	rawLine string
}

func (e ErrorCouldNotParseSymbolAndMethodName) Error() string {
	return fmt.Sprintf("could not parse out line '%d' in document with uri: %s\nrawLine: %s", e.line, e.uri, e.rawLine)
}

func (h *Handler) handleTextDocumentDefinition(request lsp.DefinitionRequest) (lsp.DefinitionResponse, error) {
	uri := request.Params.TextDocument.URI
	document, ok := h.State.Documents[uri]
	if !ok {
		return lsp.DefinitionResponse{}, ErrorDocumentDoesNotExist{uri: uri}
	}

	lines := strings.Split(document, "\n")
	curLineNum := request.Params.Position.Line
	if curLineNum > len(lines) {
		return lsp.DefinitionResponse{}, ErrorLineOutOfDocumentRange{uri: uri, line: curLineNum}
	}
	currentLine := lines[curLineNum]

	symbolName, methodName := GetSymbolAndMethodNameToLookup(currentLine, request.Params.Position)

	h.Logger.Println("symbolName:", symbolName)
	h.Logger.Println("methodName:", methodName)

	if symbolName == "" {
		symbolName = GetSymbolNameToLookup(currentLine, request.Params.Position)
	}

	if symbolName == "" && methodName == "" {
		return lsp.DefinitionResponse{}, ErrorCouldNotParseSymbolAndMethodName{uri: uri, line: curLineNum}
	}

	depsLine := GetIncludeDepsLine(document, symbolName)
	h.Logger.Println("depsLineNum:", depsLine)
	if depsLine == "" {
		h.Logger.Println("depsLineNum == \"\"")
		return lsp.DefinitionResponse{}, errors.New(fmt.Sprintf("no match found for '%s' in 'Deps' include list", symbolName))
	}

	h.Logger.Printf("depsLine: %s", depsLine)

	destinationURI, err := GetDefinitionURI(
		depsLine,
		uri,
		string(h.State.RootURI),
	)

	h.Logger.Println("destinationURI: ", destinationURI)

	if err != nil {
		h.Logger.Println("err: ", err)
		return lsp.DefinitionResponse{}, err
	}

	_, err = StatURI(destinationURI)

	if err != nil {
		h.Logger.Println("err: ", err)
		return lsp.DefinitionResponse{}, err
	}

	pos, err := GetPositionForMethodInFile(h.Logger, destinationURI, methodName)
	h.Logger.Println("err GetLineNumForMethodInFile", err)
	h.Logger.Println("pos:", pos)

	return lsp.DefinitionResponse{
		Response: lsp.Response{
			RPC: "2.0",
			ID:  &request.ID,
		},
		Result: lsp.Location{
			URI:   destinationURI,
			Range: analysis.LineRange(pos.Line, pos.Character, pos.Character),
		},
	}, nil
}

func GetIncludeDepsLine(document string, symbolName string) string {
	parser := sitter.NewParser()
	lang := ruby.GetLanguage()
	parser.SetLanguage(lang)

	tree, _ := parser.ParseCtx(
		context.Background(),
		nil,
		[]byte(document),
	)
	n := tree.RootNode()

	p := fmt.Sprintf(`
	(call
	  method: (identifier) @n_include (#eq? @n_include "include")
	  arguments: (argument_list
		       (element_reference
			 object: (constant) @n_deps (#eq? @n_deps "Deps")
			 [(string (string_content) @n_dep (#match? @n_dep "%s"))
			  (pair
			    key: (hash_key_symbol) @k (#match? @k "%s")
			    value: (string (string_content) @k_dep))])))
	`, symbolName, symbolName)

	q, _ := sitter.NewQuery([]byte(p), lang)
	qc := sitter.NewQueryCursor()
	qc.Exec(q, n)

	var capturedNode *sitter.Node

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		m = qc.FilterPredicates(m, []byte(document))
		for _, c := range m.Captures {
			if c.Index == 2 || c.Index == 4 {
				capturedNode = c.Node
			}
		}
	}

	if capturedNode == nil {
		return ""
	}

	return capturedNode.Content([]byte(document))
}

func GetSymbolNameToLookup(
	line string,
	position lsp.Position,
) string {
	parser := sitter.NewParser()
	lang := ruby.GetLanguage()
	parser.SetLanguage(lang)

	tree, _ := parser.ParseCtx(
		context.Background(),
		nil,
		[]byte(line),
	)
	n := tree.RootNode()

	p := `(_) @node`
	q, _ := sitter.NewQuery([]byte(p), lang)
	qc := sitter.NewQueryCursor()
	qc.Exec(q, n)

	var mostSpecificNode *sitter.Node
	var matchesInRange []*sitter.Node
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		// Apply predicates filtering
		m = qc.FilterPredicates(m, []byte(line))
		for _, c := range m.Captures {
			r := c.Node.Range()

			sCol := r.StartPoint.Column
			eCol := r.EndPoint.Column

			if position.Character >= int(sCol) && position.Character <= int(eCol) {
				matchesInRange = append(matchesInRange, c.Node)
			}
		}
	}

	if matchesInRange == nil {
		return ""
	}

	mostSpecificNode = matchesInRange[len(matchesInRange)-1]
	return mostSpecificNode.Content([]byte(line))
}

func GetSymbolAndMethodNameToLookup(
	line string,
	position lsp.Position,
) (string, string) {
	parser := sitter.NewParser()
	lang := ruby.GetLanguage()
	parser.SetLanguage(lang)

	tree, _ := parser.ParseCtx(
		context.Background(),
		nil,
		[]byte(line),
	)
	n := tree.RootNode()

	p := `(call receiver: (identifier) @a method: (identifier) @b)`
	q, _ := sitter.NewQuery([]byte(p), lang)
	qc := sitter.NewQueryCursor()
	qc.Exec(q, n)

	var sym, method string
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		// Apply predicates filtering
		m = qc.FilterPredicates(m, []byte(line))
		if len(m.Captures) != 2 {
			return sym, method
		}

		receiver := m.Captures[0].Node
		rangeReceiver := receiver.Range()

		methodNode := m.Captures[1].Node
		rangeMethod := methodNode.Range()

		if position.Character >= int(rangeReceiver.StartPoint.Column) && position.Character <= int(rangeMethod.EndPoint.Column) {
			sym = receiver.Content([]byte(line))
			method = methodNode.Content([]byte(line))
		}
	}

	return sym, method
}

func GetPositionForMethodInFile(logger *log.Logger, uri string, methodName string) (lsp.Position, error) {
	var pos lsp.Position

	if methodName == "" {
		return pos, nil
	}

	documentText, err := os.ReadFile(strings.TrimPrefix(uri, "file://"))
	if err != nil {
		return pos, fmt.Errorf("error GetLineNumForMethodInFile: %w", err)
	}

	parser := sitter.NewParser()
	lang := ruby.GetLanguage()
	parser.SetLanguage(lang)

	tree, _ := parser.ParseCtx(
		context.Background(),
		nil,
		[]byte(documentText),
	)
	n := tree.RootNode()

	p := fmt.Sprintf(`(method name: (identifier) @a (#match? @a ".*%s"))`, methodName)
	q, _ := sitter.NewQuery([]byte(p), lang)
	qc := sitter.NewQueryCursor()
	qc.Exec(q, n)

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		// Apply predicates filtering
		m = qc.FilterPredicates(m, []byte(documentText))
		logger.Println("captures", m.Captures)
		logger.Println("document", documentText)
		logger.Println("p", p)
		if len(m.Captures) != 1 {
			return pos, errors.New(fmt.Sprintf("unable to resolve method with name '%s' in file '%s'", methodName, uri))
		}

		pos.Line = int(m.Captures[0].Node.Range().StartPoint.Row)
		pos.Character = int(m.Captures[0].Node.Range().StartPoint.Column)
	}

	return pos, nil
}

var SliceNames = map[string]bool{
	"domain":         true,
	"collaborations": true,
}

func GetDefinitionURI(currentLine string, currentURI string, rootURI string) (string, error) {
	trimmedLine := strings.Trim(currentLine, " \",")

	firstIden, rest, found := strings.Cut(trimmedLine, ".")

	var sliceName string
	if found && SliceNames[firstIden] {
		trimmedLine = rest
		sliceName = firstIden
	} else {
		re := regexp.MustCompile(rootURI + `/slices/(\w*)/`)
		matches := re.FindStringSubmatch(currentURI)
		if len(matches) < 2 {
			return "", errors.New("unable to infer current slice name")
		}

		sliceName = re.FindStringSubmatch(currentURI)[1]
	}

	destURIExtension := strings.Replace(trimmedLine, ".", "/", -1) + ".rb"

	return rootURI + "/slices/" + sliceName + "/" + destURIExtension, nil
}

func StatURI(input string) (os.FileInfo, error) {
	filepath := strings.TrimPrefix(input, "file://")
	return os.Stat(filepath)
}

func writeResponse(writer io.Writer, msg any) {
	reply := rpc.EncodeMessage(msg)
	writer.Write([]byte(reply))
}

func getLogger(filename string) *log.Logger {
	logfile, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		panic("hey, you didn't give me a good file")
	}

	return log.New(logfile, "[hanamilsp]", log.Ldate|log.Ltime|log.Lshortfile)
}
