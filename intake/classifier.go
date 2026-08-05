package intake

import (
	"github.com/yeomyeonggeori/bluecollar/model"
)

type Classifier struct {
	languageModel model.LanguageModelProvider
}

func NewClassifier(languageModel model.LanguageModelProvider) *Classifier {
	return &Classifier{languageModel: languageModel}
}
