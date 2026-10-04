package ai

import "testing"

func TestAutocompleteBoundaries(t *testing.T) {
	for _, c := range []struct{ context, prediction, want string }{
		{"I want to spend less time", " worrying about small details. Tomorrow I will relax.", "worrying about small details."},
		{"I wonder", " why? Maybe I should ask.", "why?"},
		{"I feel", " relieved! This is a new start.", "relieved!"},
		{"I thought", " ‘this is enough.’ Then I left.", "‘this is enough.’"},
		{"I need to", " rest\n\nTomorrow is another day.", "rest"},
		{"I need to", "\nStart a paragraph", ""},
		{"I need to", " rest\r\nNew paragraph", "rest"},
		{"I walked", " 2.5 miles. Then I ran.", "2.5 miles."},
		{"I need to", " rest a little", "rest a little"},
		{"I need to", "I need to", ""},
		{"I finished.", " Now I will rest.", ""},
		{"I finished.   ", " Now I will rest.", ""},
		{"I asked why?", " Then I left.", ""},
		{"I said ‘enough.’", " Then I left.", ""},
		{"I stopped。", " Next thought", ""},
		{"I need to\n", " rest", ""},
		{"I need to\n  ", " rest", ""},
		{"", " a thought", ""},
	} {
		if got := trimAutocomplete(c.context, c.prediction); got != c.want {
			t.Errorf("%q + %q: got %q, want %q", c.context, c.prediction, got, c.want)
		}
	}
}

func TestAutocompleteRejectsEchoesAndPreservesInsertion(t *testing.T) {
	for _, c := range []struct{ context, prediction, want string }{
		{"I'm feeling burnt out and a bit overwhelmed", "I'm feeling burnt out and a bit overwhelmed.", ""},
		{"I am tired", "i am tired.", ""},
		{"I want to finish", "I want to finish this task.", ""},
		{"I need", " to stop.", " to stop."},
		{"I feel overwh", "elmed.", "elmed."},
		{"I feel overwhelmed", ".", "."},
		{"I need", " to to stop.", ""},
		{"I need", " assistant: You should relax.", ""},
		{"I need", " <|im_start|>assistant", ""},
	} {
		if got := trimAutocompleteInsertion(c.context, c.prediction); got != c.want {
			t.Errorf("%q + %q: got %q, want %q", c.context, c.prediction, got, c.want)
		}
	}
}
