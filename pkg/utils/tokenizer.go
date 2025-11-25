package utils

import (
	"fmt"
	"io"
	"strings"
)

const (
	ArtefactTypeText      = "text"
	ArtefactTypeCodeStart = "code-start"
	ArtefactTypeCodeEnd   = "code-end"

	ArtefactName = "artefact"
)

type Artifact struct {
	Content  string
	Type     string
	Metadata map[string]string
}

func Extract(s string) ([]Artifact, error) {
	artifacts := make([]Artifact, 0, 128)

	return artifacts, nil
}

// Tokenizer will parse a string and extract artifacts from it
// here is an example stream
//
// You're absolutely right! I can see the issue - the travel-agent-upsert workflow is using...
// <artefact name="travel-agent-upsert.json" contentType="application/json">
//
//	{
//	  "name": "travel-agent-upsert",
//	  "steps": []
//	}
//
// </artefact>
//
// Done !!
//
// From the example above we need 3 artefacts
// text -> code -> text -> code -> text
// Each individual xml tag will be a separate token
type Tokenizer struct {
	input []rune
	pos   int
}

func NewTokenizer(input string) *Tokenizer {
	return &Tokenizer{
		input: []rune(input),
	}
}

func (t *Tokenizer) NextToken() (Artifact, error) {
	t.clearWhiteSpace()

	switch {
	case !t.hasNext():
		return Artifact{}, io.EOF
	case t.next() == '<':
		return t.artefact()
	default:
		s, err := t.text()
		if err != nil {
			return Artifact{}, err
		}

		return Artifact{
			Content: s,
			Type:    ArtefactTypeText,
		}, nil
	}
}

// next returns the next rune in the input but not advancing the position
func (t *Tokenizer) next() rune {
	return t.input[t.pos]
}

// hasNext returns true if there are more runes to read
func (t *Tokenizer) hasNext() bool {
	return t.pos < len(t.input)
}

var whitespace = map[rune]bool{' ': true, '\t': true, '\n': true, '\r': true}

// clearWhiteSpace will move the position to the next non-whitespace character
func (t *Tokenizer) clearWhiteSpace() {
	for t.hasNext() && whitespace[t.next()] {
		t.pos++
	}
}

func (t *Tokenizer) text() (string, error) {
	defer t.clearWhiteSpace()

	startPos := t.pos
	i := t.pos
	for ; i < len(t.input); i++ {
		if t.input[i] == '<' {
			t.pos = i

			return string(t.input[startPos:i]), nil
		}
	}

	if i == len(t.input) {
		t.pos = i

		return string(t.input[startPos:i]), nil
	}

	return "", fmt.Errorf("no text found at possition %d", t.pos)
}

func (t *Tokenizer) artefact() (Artifact, error) {
	defer t.clearWhiteSpace()

	startPos := t.pos
	for i := t.pos; i < len(t.input); i++ {
		if t.input[i] == '>' {
			t.pos = i + 1

			return t.parseArtefact(string(t.input[startPos : i+1]))
		}
	}

	return Artifact{}, fmt.Errorf("no artefact found at possition %d", t.pos)
}

// parsesArtefact will turn a string like this <artefact name="travel-agent-upsert.json" contentType="application/json">
// into an Artifact
func (t *Tokenizer) parseArtefact(input string) (Artifact, error) {
	if len(input) < 2 {
		return Artifact{}, fmt.Errorf("invalid code artefact artefact: %s", input)
	}

	artefactType := ArtefactTypeCodeStart

	// part 1 is the tag name and right now we only have 1 called artefact so we disregard it
	parts := strings.Split(input, " ")

	switch input[1] {
	case '/':
		artefactType = ArtefactTypeCodeEnd
	default:
	}

	metadata := map[string]string{}
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}

		kv := strings.Split(p, "=")
		if len(kv) != 2 {
			return Artifact{}, fmt.Errorf("invalid artefact metadata: %s", p)
		}

		metadata[kv[0]] = strings.Trim(kv[1], "\"'")
	}

	return Artifact{
		Type:     artefactType,
		Metadata: metadata,
	}, nil
}
