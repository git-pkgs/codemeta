package codemeta_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestConcurrentParsingAndValidation(t *testing.T) {
	input := []byte(`{"@context":"https://w3id.org/codemeta/3.0","name":"Example","author":[{"givenName":"Ada","familyName":"Lovelace"}],"datePublished":"2023-02-29","unknown":{"nested":true}}`)
	doc, err := codemeta.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	want := doc.Validate()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 50 {
				separate, err := codemeta.Parse(input)
				if err != nil {
					t.Error(err)
					return
				}
				if !reflect.DeepEqual(separate.Validate(), want) || !reflect.DeepEqual(doc.Validate(), want) {
					t.Error("diagnostics changed")
				}
				fields := doc.Get("author").Items()[0].Fields()
				fields[0].Name = "changed"
				diagnostics := doc.Validate()
				diagnostics[0].Code = "changed"
				if doc.Author()[0].GivenName() != "Ada" || doc.Version() != codemeta.Version3 {
					t.Error("metadata changed")
				}
			}
		})
	}
	wg.Wait()
}
