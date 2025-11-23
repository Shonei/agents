package utils

type ContentTypePart string

const (
	ContentTypePartText      ContentTypePart = "text"
	ContentTypePartCode      ContentTypePart = "code"
	ContentTypePartUndefined ContentTypePart = "undefined"
)

type ContentParts struct {
	Content string
	Meta    map[string]string
	Type    string
}

// func ParseContent(content string) ([]ContentParts, error) {
// 	tokenizer := NewTokenizer(content)
// 	parts := make([]ContentParts, 0, 128)
//
// 	var nextpartType ContentTypePart
//
// 	for {
// 		token, err := tokenizer.NextToken()
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		switch nextpartType {
// 		case ContentTypePartText:
//
// 			parts = append(parts, ContentParts{
// 				Content: "token",
// 				Type:    string(ContentTypePartText),
// 			})
// 			continue
// 		case ContentTypePartCode:
// 			token, err := tokenizer.artefact()
// 			if err != nil {
// 				return nil, err
// 			}
//
// 			parts = append(parts, ContentParts{
// 				Content: token.Content,
// 				Meta:    token.Metadata,
// 				Type:    string(ContentTypePartCode),
// 			})
// 			continue
// 		case ContentTypePartUndefined:
// 			token, err := tokenizer.NextToken()
// 			if err != nil {
// 				return nil, err
// 			}
//
// 			switch token.Type {
// 			case ArtefactTypeText:
// 				// anything can be after text
// 				nextpartType = ContentTypePartUndefined
// 			case ArtefactTypeCodeStart:
// 				// only text can be after code start
// 				nextpartType = ContentTypePartText
// 			case ArtefactTypeCodeEnd:
// 				// anything can be after code end
// 				nextpartType = ContentTypePartUndefined
// 			default:
// 				return nil, fmt.Errorf("unknown token type: %s", token.Type)
// 			}
//
// 			parts = append(parts, ContentParts{
// 				Content: token.Content,
// 				Meta:    token.Metadata,
// 				Type:    token.Type,
// 			})
// 			continue
// 		default:
// 			return nil, fmt.Errorf("unknown next part type: %s", nextpartType)
// 		}
// 	}
//
// 	return parts, nil
// }
