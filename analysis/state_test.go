package analysis

import (
	"hanamilsp/lsp"
	"log"
	"os"
	"regexp"
	"testing"

	"github.com/matryer/is"
)

func TestGetDefinitionURI(t *testing.T) {
	is := is.New(t)
	input := "           \"operations.commands.services.validation_goal_publishable\","
	currentURI := "file:///Users/ayden.aba/Documents/ca/code/goals-service/slices/domain/operations/commands/create_published_goal.rb"
	rootURI := "file:///Users/ayden.aba/Documents/ca/code/goals-service"

	expectedURI := rootURI + "/slices/domain/operations/commands/services/validation_goal_publishable.rb"
	state := NewState(
		log.New(os.Stdout, "test", 1),
	)
	state.RootURI = lsp.DocumentURI(rootURI)

	receivedURI, _ := state.GetDefinitionURI(input, currentURI, rootURI)

	is.Equal(receivedURI, expectedURI)
}

func TestRegexp(t *testing.T) {
	rootURI := "file:///Users/ayden.aba/Documents/ca/code/goals-service"
	currentURI := "file:///Users/ayden.aba/Documents/ca/code/goals-service/slices/domain/operations/commands/create_published_goal.rb"
	reStr := rootURI + "/slices/(.*?)/"
	t.Log(reStr)
	re := regexp.MustCompile(rootURI + `/slices/(\w*)/`)
	t.Log(re.FindStringSubmatch(currentURI))
}
