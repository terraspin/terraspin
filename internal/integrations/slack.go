package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SlackWebhook sends analysis notifications via Slack Incoming Webhook.
type SlackWebhook struct {
	WebhookURL string
}

// slackBlock represents a Block Kit block.
type slackBlock struct {
	Type   string      `json:"type"`
	Text   *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SendRiskNotification posts a block-kit message to Slack.
func (s *SlackWebhook) SendRiskNotification(tier string, score float64, summary string, rcCounts map[string]int) error {
	if s.WebhookURL == "" {
		return fmt.Errorf("slack: empty webhook URL")
	}

	emoji := map[string]string{"critical": "🚨", "high": "⚠️", "medium": "🔶", "low": "✅"}
	e := emoji[strings.ToLower(tier)]
	if e == "" {
		e = "🌀"
	}

	var fields []slackText
	for _, action := range []string{"create", "update", "delete", "replace"} {
		if c := rcCounts[action]; c > 0 {
			fields = append(fields, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*%s:* %d", action, c)})
		}
	}

	msg := struct {
		Blocks []slackBlock `json:"blocks"`
	}{
		Blocks: []slackBlock{
			{Type: "header", Text: &slackText{Type: "plain_text", Text: fmt.Sprintf("%s Terraspin: %s risk (%.0f)", e, strings.ToUpper(tier), score)}},
			{Type: "section", Fields: fields},
		},
	}
	if summary != "" {
		msg.Blocks = append(msg.Blocks, slackBlock{Type: "section", Text: &slackText{Type: "mrkdwn", Text: summary}})
	}

	raw, _ := json.Marshal(msg)
	resp, err := http.Post(s.WebhookURL, "application/json", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("slack: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack: HTTP %d", resp.StatusCode)
	}
	return nil
}
