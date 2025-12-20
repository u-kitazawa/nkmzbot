package transcribe

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	client *openai.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		client: openai.NewClient(apiKey),
	}
}

func (c *Client) Transcribe(ctx context.Context, filePath string) (string, error) {
	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: filePath,
	}
	resp, err := c.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}
