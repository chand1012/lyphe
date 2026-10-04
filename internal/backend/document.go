package backend

import (
	"encoding/json"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/net/html"
	"strings"
)

type Node map[string]any
type Document struct {
	Version      int      `json:"version"`
	Value        []Node   `json:"value"`
	AudioFileIDs []string `json:"audioFileIds"`
	Media        []Node   `json:"media,omitempty"`
}
type Reference struct{ Kind, ID string }
type Placement struct{ File, ID, Location string }

var collections = map[string]string{"journal": "journal", "task": "tasks", "goal": "goals", "habit": "habits"}
var allowedNodes = map[string]bool{"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "blockquote": true, "hr": true, "list-item": true, "ul": true, "ol": true, "li": true, "code_block": true, "code_line": true, "a": true, "entity-mention": true, "file-attachment": true, "image": true, "video": true}

func ReadDocument(r *core.Record) Document {
	var d Document
	_ = r.UnmarshalJSONField("content", &d)
	if d.Version == 0 {
		d = legacyDocument(r.GetString("description"))
	}
	return d
}
func legacyDocument(source string) Document {
	root, _ := html.Parse(strings.NewReader(source))
	value := []Node{}
	var walk func(*html.Node, map[string]any)
	walk = func(n *html.Node, marks map[string]any) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		if n.Type == html.TextNode {
			if n.Data != "" {
				leaf := Node{"text": n.Data}
				for k, v := range marks {
					leaf[k] = v
				}
				if len(value) == 0 {
					value = append(value, Node{"type": "p", "children": []any{}})
				}
				last := value[len(value)-1]
				last["children"] = append(last["children"].([]any), leaf)
			}
			return
		}
		next := map[string]any{}
		for k, v := range marks {
			next[k] = v
		}
		switch n.Data {
		case "strong", "b":
			next["bold"] = true
		case "i", "em":
			next["italic"] = true
		case "u":
			next["underline"] = true
		case "s", "del":
			next["strikethrough"] = true
		case "code":
			next["code"] = true
		}
		if n.Type == html.ElementNode {
			typ := n.Data
			switch typ {
			case "div", "p":
				typ = "p"
			case "h1", "h2", "h3", "h4", "h5", "h6", "blockquote":
			default:
				typ = ""
			}
			if typ != "" {
				value = append(value, Node{"type": typ, "children": []any{}})
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, next)
		}
	}
	if root != nil {
		walk(root, map[string]any{})
	}
	for _, n := range value {
		if len(n["children"].([]any)) == 0 {
			n["children"] = []any{Node{"text": ""}}
		}
	}
	if len(value) == 0 {
		value = []Node{{"type": "p", "children": []any{Node{"text": ""}}}}
	}
	return Document{Version: 1, Value: value, AudioFileIDs: []string{}}
}
func owned(app core.App, table, id, user string) (*core.Record, error) {
	r, err := app.FindRecordById(table, id)
	if err != nil || r.GetString("user") != user {
		return nil, fmt.Errorf("record is unavailable")
	}
	return r, nil
}
func ValidateDocument(app core.App, d Document, user string) (string, []Reference, []Placement, error) {
	raw, _ := json.Marshal(d)
	if len(raw) > 1048576 || d.Version != 1 || len(d.Value) == 0 {
		return "", nil, nil, fmt.Errorf("invalid document version, size, or body")
	}
	refs := []Reference{}
	placements := []Placement{}
	var text strings.Builder
	count := 0
	seen := map[string]bool{}
	var walk func(Node, int) error
	walk = func(n Node, depth int) error {
		count++
		if depth > 32 || count > 10000 {
			return fmt.Errorf("document is too complex")
		}
		if s, ok := n["text"].(string); ok {
			text.WriteString(s)
			return nil
		}
		typ, _ := n["type"].(string)
		if !allowedNodes[typ] {
			return fmt.Errorf("unsupported node %q", typ)
		}
		if typ == "entity-mention" {
			kind, _ := n["kind"].(string)
			id, _ := n["entityId"].(string)
			table, ok := collections[kind]
			if !ok {
				return fmt.Errorf("invalid mention")
			}
			r, err := owned(app, table, id, user)
			if err != nil {
				return err
			}
			refs = append(refs, Reference{kind, id})
			text.WriteString(r.GetString("title"))
		}
		if typ == "file-attachment" || typ == "image" || typ == "video" {
			id, _ := n["fileId"].(string)
			r, err := owned(app, "files", id, user)
			if err != nil {
				return fmt.Errorf("invalid file reference")
			}
			pid, _ := n["placementId"].(string)
			if pid == "" {
				return fmt.Errorf("missing file placement id")
			}
			if seen[pid] {
				return fmt.Errorf("duplicate file placement")
			}
			seen[pid] = true
			placements = append(placements, Placement{id, pid, typ})
			text.WriteString(r.GetString("name"))
			delete(n, "url")
		}
		if typ == "a" {
			if u, _ := n["url"].(string); u != "" && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "mailto:") {
				return fmt.Errorf("invalid link")
			}
		}
		children, ok := n["children"].([]any)
		if !ok || len(children) == 0 {
			return fmt.Errorf("node requires children")
		}
		for _, child := range children {
			m, ok := child.(map[string]any)
			if !ok {
				if node, yes := child.(Node); yes {
					m = map[string]any(node)
					ok = true
				}
			}
			if !ok {
				return fmt.Errorf("invalid child")
			}
			if err := walk(Node(m), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, n := range d.Value {
		if err := walk(n, 0); err != nil {
			return "", nil, nil, err
		}
		text.WriteString("\n")
	}
	for _, id := range d.AudioFileIDs {
		if _, err := owned(app, "files", id, user); err != nil {
			return "", nil, nil, err
		}
		placements = append(placements, Placement{id, id, "audio"})
	}
	for _, n := range d.Media {
		if err := walk(n, 0); err != nil {
			return "", nil, nil, err
		}
	}
	return text.String(), refs, placements, nil
}
