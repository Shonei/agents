package index

type Chunk struct {
	Content string
	Meta    map[string]string
}

type Strategy func(path string, content string) ([]Chunk, error)
