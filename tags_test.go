package lamvms

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	"go.uber.org/mock/gomock"
)

func TestSyncTags_NoChanges(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListTags(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListTagsOutput{
			Tags: map[string]string{"env": "dev", "team": "infra"},
		}, nil)

	app := &App{client: mock}
	if err := app.syncTags(context.Background(), testImageARN, map[string]string{"env": "dev", "team": "infra"}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncTags_AddAndUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListTags(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListTagsOutput{
			Tags: map[string]string{"env": "dev"},
		}, nil)

	mock.EXPECT().
		TagResource(gomock.Any(), gomock.Eq(&lambdamicrovms.TagResourceInput{
			Resource: aws.String(testImageARN),
			Tags:     map[string]string{"env": "prod", "team": "infra"},
		})).
		Return(&lambdamicrovms.TagResourceOutput{}, nil)

	app := &App{client: mock}
	if err := app.syncTags(context.Background(), testImageARN, map[string]string{"env": "prod", "team": "infra"}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncTags_Remove(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListTags(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListTagsOutput{
			Tags: map[string]string{"env": "dev", "old-tag": "remove-me"},
		}, nil)

	mock.EXPECT().
		UntagResource(gomock.Any(), gomock.Eq(&lambdamicrovms.UntagResourceInput{
			Resource: aws.String(testImageARN),
			TagKeys:  []string{"old-tag"},
		})).
		Return(&lambdamicrovms.UntagResourceOutput{}, nil)

	app := &App{client: mock}
	if err := app.syncTags(context.Background(), testImageARN, map[string]string{"env": "dev"}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncTags_AddAndRemove(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListTags(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListTagsOutput{
			Tags: map[string]string{"env": "dev", "old": "value"},
		}, nil)

	mock.EXPECT().
		UntagResource(gomock.Any(), gomock.Eq(&lambdamicrovms.UntagResourceInput{
			Resource: aws.String(testImageARN),
			TagKeys:  []string{"old"},
		})).
		Return(&lambdamicrovms.UntagResourceOutput{}, nil)

	mock.EXPECT().
		TagResource(gomock.Any(), gomock.Eq(&lambdamicrovms.TagResourceInput{
			Resource: aws.String(testImageARN),
			Tags:     map[string]string{"env": "prod", "new": "tag"},
		})).
		Return(&lambdamicrovms.TagResourceOutput{}, nil)

	app := &App{client: mock}
	if err := app.syncTags(context.Background(), testImageARN, map[string]string{"env": "prod", "new": "tag"}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncTags_NilDesired(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockLambdaMicroVMsClient(ctrl)

	mock.EXPECT().
		ListTags(gomock.Any(), gomock.Any()).
		Return(&lambdamicrovms.ListTagsOutput{
			Tags: map[string]string{"env": "dev"},
		}, nil)

	mock.EXPECT().
		UntagResource(gomock.Any(), gomock.Eq(&lambdamicrovms.UntagResourceInput{
			Resource: aws.String(testImageARN),
			TagKeys:  []string{"env"},
		})).
		Return(&lambdamicrovms.UntagResourceOutput{}, nil)

	app := &App{client: mock}
	if err := app.syncTags(context.Background(), testImageARN, nil); err != nil {
		t.Fatal(err)
	}
}
