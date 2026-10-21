package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type bedrockProvider struct{}

func init() { registerProvider(bedrockProvider{}) }

func (bedrockProvider) Name() string   { return "bedrock" }
func (bedrockProvider) NeedsKey() bool { return false }
func (bedrockProvider) KeyEnv() string { return "" }

func (bedrockProvider) Query(ctx context.Context, request Request) (Response, error) {
	if request.Model == "" {
		return Response{}, fmt.Errorf("bedrock: model is required; set --model or llm.model")
	}

	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRetryMaxAttempts(1))
	if err != nil {
		return Response{}, bedrockConfigError(err)
	}
	if awsConfig.Region == "" {
		return Response{}, bedrockConfigError(errors.New("missing AWS region"))
	}
	if _, err := awsConfig.Credentials.Retrieve(ctx); err != nil {
		return Response{}, bedrockConfigError(err)
	}

	output, err := bedrockruntime.NewFromConfig(awsConfig).Converse(ctx, &bedrockruntime.ConverseInput{
		ModelId: aws.String(request.Model),
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: request.System},
		},
		Messages: []types.Message{{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: request.Prompt},
			},
		}},
		InferenceConfig: &types.InferenceConfiguration{MaxTokens: aws.Int32(1024)},
	})
	if err != nil {
		var responseErr *smithyhttp.ResponseError
		if errors.As(err, &responseErr) {
			return Response{}, &HTTPError{Provider: "bedrock", StatusCode: responseErr.HTTPStatusCode(), Message: responseErr.Error()}
		}
		return Response{}, fmt.Errorf("bedrock: converse: %w", err)
	}

	message, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return Response{}, fmt.Errorf("bedrock: no message in response")
	}
	for _, block := range message.Value.Content {
		if text, ok := block.(*types.ContentBlockMemberText); ok {
			tokens := 0
			if output.Usage != nil {
				tokens = int(aws.ToInt32(output.Usage.TotalTokens))
			}
			return Response{Text: text.Value, Tokens: tokens}, nil
		}
	}
	return Response{}, fmt.Errorf("bedrock: no text content in response")
}

func bedrockConfigError(err error) error {
	return fmt.Errorf("bedrock: could not load AWS configuration: %w; set AWS_REGION and credentials", err)
}
