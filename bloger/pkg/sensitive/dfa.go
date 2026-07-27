package sensitive

type Filter interface {
	Match(text string) []string
}

// DFA 敏感词过滤器
type DFA struct {
	root *trieNode
}

type trieNode struct {
	children map[rune]*trieNode
	isEnd    bool
}

func New() *DFA {
	return &DFA{root: &trieNode{children: make(map[rune]*trieNode)}}
}

func (f *DFA) AddWords(words ...string) {
	for _, w := range words {
		f.addWord(w)
	}
}

func (f *DFA) addWord(word string) {
	node := f.root
	for _, ch := range word {
		if node.children[ch] == nil {
			node.children[ch] = &trieNode{children: make(map[rune]*trieNode)}
		}
		node = node.children[ch]
	}
	node.isEnd = true
}

// Match 返回文本中匹配到的所有敏感词（最长匹配）
func (f *DFA) Match(text string) []string {
	if text == "" {
		return nil
	}

	var result []string
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		node := f.root
		found := false
		endPos := 0

		for j := i; j < len(runes); j++ {
			child, ok := node.children[runes[j]]
			if !ok {
				break
			}
			node = child
			if node.isEnd {
				found = true
				endPos = j
			}
		}

		if found {
			word := string(runes[i : endPos+1])
			result = append(result, word)
			i = endPos
		}
	}

	return result
}
