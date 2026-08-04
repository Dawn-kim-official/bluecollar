package intake

import (
	"github.com/Dawn-kim-official/bluecollar/model"
)

type Classifier struct {
	languageModel model.LanguageModelProvider
}

func NewClassifier(languageModel model.LanguageModelProvider) *Classifier {
	return &Classifier{languageModel: languageModel}
}
