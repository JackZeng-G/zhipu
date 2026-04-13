package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// NoteInfo is a lightweight note descriptor for classification.
type NoteInfo struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	NotebookName string `json:"notebook_name,omitempty"`
	Summary      string `json:"summary,omitempty"`
}

// CategoryNode represents a node in the LLM-generated category tree.
type CategoryNode struct {
	Name     string         `json:"name"`
	NoteIDs  []string       `json:"note_ids"`
	Children []CategoryNode `json:"children,omitempty"`
}

// BuildCategoryTree uses an LLM to generate a hierarchical category tree for all notes.
func BuildCategoryTree(ctx context.Context, provider Provider, notes []NoteInfo) ([]CategoryNode, error) {
	if len(notes) == 0 {
		return nil, fmt.Errorf("no notes to categorize")
	}

	// Build the note list for the prompt
	var noteLines []string
	for _, n := range notes {
		line := fmt.Sprintf("- ID: %s, 标题: %s", n.ID, n.Title)
		if n.NotebookName != "" {
			line += fmt.Sprintf(", 笔记本: %s", n.NotebookName)
		}
		if n.Summary != "" {
			s := n.Summary
			if len(s) > 100 {
				s = s[:100] + "..."
			}
			line += fmt.Sprintf(", 摘要: %s", s)
		}
		noteLines = append(noteLines, line)
	}

	prompt := fmt.Sprintf(`你是一个知识管理专家。以下是一组笔记的列表，请为它们设计一个树形分类体系。

要求：
1. 最多 3 层深度（根→分类→子分类）
2. 每个叶子分类包含 2-15 篇笔记
3. 分类名称简洁（2-8 个字），使用中文
4. 每篇笔记至少属于一个分类，可以属于多个
5. 分类要覆盖所有笔记，不要遗漏

笔记列表：
%s

请返回 JSON 数组格式（不要 markdown 代码块，直接返回 JSON）：
[{"name":"分类名","note_ids":["id1","id2"],"children":[{"name":"子分类名","note_ids":["id3"]}]}]`, strings.Join(noteLines, "\n"))

	resp, err := provider.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generate category tree: %w", err)
	}

	// Parse the response
	cleaned := cleanJSONResponse(resp)
	var nodes []CategoryNode
	if err := json.Unmarshal([]byte(cleaned), &nodes); err != nil {
		log.Printf("[categorizer] failed to parse category tree response: %v\nraw: %s", err, resp)
		return nil, fmt.Errorf("parse category tree: %w", err)
	}

	log.Printf("[categorizer] generated category tree with %d root categories", len(nodes))
	return nodes, nil
}

// ClassifyNote classifies a single note into existing categories using LLM.
func ClassifyNote(ctx context.Context, provider Provider, note NoteInfo, categoryPaths []string) ([]string, error) {
	if len(categoryPaths) == 0 {
		return nil, nil
	}

	prompt := fmt.Sprintf(`你是一个知识管理专家。请将以下笔记归入最合适的分类中。

笔记标题: %s
笔记摘要: %s

现有分类列表：
%s

请返回笔记应该归入的分类路径（JSON 数组，最多3个，按相关度排序）。
只返回 JSON 数组，不要其他内容：
["/分类1/子分类", "/分类2"]`, note.Title, note.Summary, strings.Join(categoryPaths, "\n"))

	resp, err := provider.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM classify note: %w", err)
	}

	cleaned := cleanJSONResponse(resp)
	var paths []string
	if err := json.Unmarshal([]byte(cleaned), &paths); err != nil {
		log.Printf("[categorizer] failed to parse classify response: %v", err)
		return nil, nil
	}
	return paths, nil
}

// cleanJSONResponse extracts JSON from a potentially markdown-wrapped response.
func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)
	// Remove markdown code blocks
	if strings.HasPrefix(s, "```") {
		// Find the end of the first line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}
