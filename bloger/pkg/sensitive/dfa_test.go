package sensitive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDFA_Match_SingleWord(t *testing.T) {
	f := New()
	f.AddWords("敏感词")

	result := f.Match("这是一段包含敏感词的文本")
	assert.Len(t, result, 1)
	assert.Equal(t, "敏感词", result[0])
}

func TestDFA_Match_MultipleWords(t *testing.T) {
	f := New()
	f.AddWords("敏感词", "违禁词")

	result := f.Match("敏感词和违禁词都出现了")
	assert.Len(t, result, 2)
}

func TestDFA_Match_NoMatch(t *testing.T) {
	f := New()
	f.AddWords("敏感词")

	result := f.Match("这是一段正常文本")
	assert.Empty(t, result)
}

func TestDFA_Match_Chinese(t *testing.T) {
	f := New()
	f.AddWords("赌博", "色情")

	result := f.Match("禁止传播色情和赌博内容")
	assert.Len(t, result, 2)
}

func TestDFA_Match_EmptyText(t *testing.T) {
	f := New()
	f.AddWords("敏感词")

	result := f.Match("")
	assert.Empty(t, result)
}

func TestDFA_Match_Substring(t *testing.T) {
	f := New()
	f.AddWords("ab", "abc")

	// 最长匹配原则："abc" 中的 "ab" 不重复匹配
	result := f.Match("abc")
	assert.Len(t, result, 1)
	assert.Equal(t, "abc", result[0])
}

func TestDFA_EmptyFilter(t *testing.T) {
	f := New()

	result := f.Match("任何文本")
	assert.Empty(t, result)
}
