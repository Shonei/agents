package utils

import (
	"io"
	"testing"
)

func Test_parse(t *testing.T) {
	const input = `
	<artefact name="travel-agent-upsert.json" contentType="application/json">
	{
	  "name": "travel-agent-upsert",
	  "steps": []
	}
	</artefact>
	some more words
	<artefact name="travel-agent-upsert.json" contentType="application/json">
	{
	  "name": "travel-agent-upsert",
	  "steps": []
	}
	</artefact>
	`

	tokens := NewTokenizer(input)
	for {
		token, err := tokens.NextToken()
		if err != nil {
			if err == io.EOF {
				break
			}

			t.Fatal(err)
		}

		t.Logf("Token: %+v", token)
	}
}
